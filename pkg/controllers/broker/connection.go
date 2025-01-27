/*
 * Copyright 2020 Mandelsoft. All rights reserved.
 *  This file is licensed under the Apache Software License, v. 2 except as noted
 *  otherwise in the LICENSE file
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package broker

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller/reconcile"
	"github.com/gardener/controller-manager-library/pkg/logger"
	"github.com/gardener/controller-manager-library/pkg/utils"
	"golang.org/x/net/ipv4"

	"github.com/mandelsoft/kubelink/pkg/kubelink"
	"github.com/mandelsoft/kubelink/pkg/tcp"
)

const BufferSize = 17000

// Packet types:
// 0: Normal data payload
// 1: Hello message
// More types planned for intermediate transfer of meta information
// Unknown packets have to be skipped and returned with reject bit set

const PACKET_TYPE_DATA = 0
const PACKET_TYPE_HELLO = 1

////////////////////////////////////////////////////////////////////////////////

type ConnectionFailHandler interface {
	Notify(*TunnelConnection, error)
}

type TunnelConnection struct {
	logger.LogContext
	lock          sync.RWMutex
	mux           *Mux
	conn          net.Conn
	clusterCIDR   *net.IPNet
	remoteAddress string
	handlers      []ConnectionFailHandler

	wlock sync.Mutex
	rlock sync.Mutex
}

func NewTunnelConnection(mux *Mux, conn net.Conn, link *kubelink.Link, handlers ...ConnectionFailHandler) (*TunnelConnection, *ConnectionHello, error) {
	remote := conn.RemoteAddr().String()
	t := &TunnelConnection{
		LogContext:    mux.NewContext("source", remote),
		mux:           mux,
		conn:          conn,
		remoteAddress: remote,
		handlers:      append(handlers[:0:0], handlers...),
	}
	if link != nil {
		t.clusterCIDR = link.ClusterAddress
	}

	hello, err := t.handshake()
	if err != nil {
		return nil, nil, err
	}
	if hello != nil {
		cidr := hello.GetClusterCIDR()
		if !net.IPv6zero.Equal(cidr.IP) {
			if link != nil {
				if !link.ClusterAddress.IP.Equal(cidr.IP) {
					return nil, hello, fmt.Errorf("cluster address mismatch: got %s but expected %s", cidr.IP, link.ClusterAddress.IP)
				}
			}
			if !cidr.Contains(mux.clusterAddr.IP) {
				// obsolete when we support unidirectional connections
				return nil, hello, fmt.Errorf("cluster address mismatch: own address %s not in foreign range", mux.clusterAddr.IP, cidr)
			}
			if !mux.clusterAddr.Contains(cidr.IP) {
				return nil, hello, fmt.Errorf("cluster address mismatch: remote address %s not in local range", cidr.IP, mux.clusterAddr)
			}
		}
		if mux.connectionHandler != nil {
			t.Infof("start hello handling....")
			go mux.connectionHandler.UpdateAccess(hello)
		}
	}
	return t, hello, nil
}

func (this *TunnelConnection) String() string {
	return fmt.Sprintf("%s[%s]", this.clusterCIDR, this.remoteAddress)
}

func (this *TunnelConnection) RegisterStateHandler(handlers ...ConnectionFailHandler) {
	this.lock.Lock()
	defer this.lock.Unlock()

	this.handlers = append(this.handlers, handlers...)
}

func (this *TunnelConnection) notify(err error) {
	if err == io.EOF {
		return
	}
	this.mux.Notify(this, err)
	this.lock.RLock()
	defer this.lock.RUnlock()
	for _, h := range this.handlers {
		h.Notify(this, err)
	}
}

func printConnState(log logger.LogContext, state tls.ConnectionState) {
	log.Info(">>>>>>>>>>>>>>>> State <<<<<<<<<<<<<<<<")
	log.Infof("Version: %x", state.Version)
	log.Infof("HandshakeComplete: %t", state.HandshakeComplete)
	log.Infof("DidResume: %t", state.DidResume)
	log.Infof("CipherSuite: %x", state.CipherSuite)
	log.Infof("NegotiatedProtocol: %s", state.NegotiatedProtocol)
	log.Infof("NegotiatedProtocolIsMutual: %t", state.NegotiatedProtocolIsMutual)

	log.Info("Certificate chain:")
	for i, cert := range state.PeerCertificates {
		subject := cert.Subject
		issuer := cert.Issuer
		log.Infof(" %d s:/C=%v/ST=%v/L=%v/O=%v/OU=%v/CN=%s", i, subject.Country, subject.Province, subject.Locality, subject.Organization, subject.OrganizationalUnit, subject.CommonName)
		log.Infof("   i:/C=%v/ST=%v/L=%v/O=%v/OU=%v/CN=%s", issuer.Country, issuer.Province, issuer.Locality, issuer.Organization, issuer.OrganizationalUnit, issuer.CommonName)
	}
	log.Info(">>>>>>>>>>>>>>>> State End <<<<<<<<<<<<<<<<")
}

////////////////////////////////////////////////////////////////////////////////

func (this *TunnelConnection) Close() error {
	return this.conn.Close()
}

func (this *TunnelConnection) writeHello(hello *ConnectionHello) error {
	data := hello.Data()
	return this.WritePacket(PACKET_TYPE_HELLO, data)
}

func (this *TunnelConnection) readHello() (*ConnectionHello, error) {
	var buffer [BufferSize]byte
	n, ty, err := this.ReadPacket(buffer[:])
	if err != nil {
		return nil, err
	}
	if ty != PACKET_TYPE_HELLO {
		return nil, fmt.Errorf("unexpected packet %d instead of hello handshake", ty)
	}
	return this.parseHelloPacket(buffer[:n])
}

func (this *TunnelConnection) parseHelloPacket(data []byte) (*ConnectionHello, error) {
	var header ConnectionHelloHeader
	if len(data) < len(header) {
		return nil, fmt.Errorf("hello packet too short (%d expected %d)", len(data), len(header))
	}
	copy(header[:], data)
	hello, err := ParseConnectionHello(this.mux, &header, data[len(header):])
	if err != nil {
		this.Errorf("invalid hello packet: %s", err)
		return nil, err
	}
	this.Infof("hello packet with %d extensions", len(hello.Extensions))
	return hello, nil
}

func (this *TunnelConnection) createHello() *ConnectionHello {
	hello := NewConnectionHello()
	hello.SetClusterCIDR(this.mux.clusterAddr)
	hello.SetPort(this.mux.port)
	if len(this.mux.local) > 0 {
		hello.SetCIDR(this.mux.local[0])
	}
	lock.RLock()
	defer lock.RUnlock()
	for _, h := range registry {
		h.Add(hello, this.mux)
	}
	return hello
}

func (this *TunnelConnection) handshake() (*ConnectionHello, error) {
	local := this.createHello()

	var werr error
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		werr = this.writeHello(local)
	}()

	remote, rerr := this.readHello()
	wg.Wait()
	if rerr != nil {
		return nil, fmt.Errorf("cannot finish connection handshake: %s", rerr)
	}
	if werr != nil {
		return nil, fmt.Errorf("cannot finish connection handshake: %s", werr)
	}
	this.Infof("REMOTE SIDE: cluster %s, net: %s port: %d", remote.GetClusterCIDR(), remote.GetCIDR(), remote.GetPort())
	return remote, nil
}

func (this *TunnelConnection) Serve() error {
	err := this.serve()
	this.notify(err)
	return err
}

func (this *TunnelConnection) serve() error {
	var buffer [BufferSize]byte
	for {
		n, ty, err := this.ReadPacket(buffer[:])
		if n < 0 || err != nil {
			this.Infof("connection aborted: %d bytes, err=%s", n, err)
			if n <= 0 {
				err = io.EOF
			}
			return err
		}
		if n == 0 {
			continue
		}
		packet := buffer[:n]
		if ty != PACKET_TYPE_DATA {
			this.Infof("got packet of unknown type %x", ty)
			continue
		}
		vers := int(packet[0]) >> 4
		if vers == ipv4.Version {
			header, err := ipv4.ParseHeader(packet)
			if err != nil {
				this.Errorf("err: %s", err)
				continue
			} else {
				this.Infof("receiving ipv4[%d]: (%d) hdr: %d, total: %d, prot: %d,  %s->%s\n",
					header.Version, len(packet), header.Len, header.TotalLen, header.Protocol, header.Src, header.Dst)
				if this.mux.clusterAddr.Contains(header.Src) {
					l := this.mux.links.GetLinkForClusterAddress(header.Src)
					if l == nil {
						this.Warnf("  dropping packet because of unknown cluster siurce address [%s]", header.Src)
						continue
					}
					granted, set := l.AllowIngress(header.Dst)
					if !granted {
						this.Warnf("  dropping packet because of non-matching destination address %s for cluster address %s", header.Dst, header.Src)
						continue
					}
					if !set && this.mux.local.IsSet() && !this.mux.local.Contains(header.Dst) {
						this.Warnf("  dropping packet because of non-matching destination address %s for cluster %s", header.Dst, header.Src)
						continue
					}
				} else {
					if !header.Dst.Equal(this.mux.clusterAddr.IP) {
						this.Warnf("  dropping packet because of non-matching destination address [%s<>%s]", this.mux.clusterAddr.IP, header.Dst)
						continue
					}
				}
			}
		}
		o, err := this.mux.tun.Write(buffer[:n])
		if err != nil {
			if err != io.EOF {
				this.Infof("connection aborted: cannot write tun: %s", err)
			}
			return err
		}
		if n != o {
			panic(fmt.Errorf("packet length %d, but written %d", n, o))
		}
	}
}

func (this *TunnelConnection) read(r io.Reader, data []byte) error {
	start := 0
	for start < len(data) {
		n, err := r.Read(data[start:])
		if err != nil {
			return err
		}
		if n < 0 {
			return io.EOF
		}
		start += n
		if start < len(data) {
			this.Infof("read next chunk %d", start)
		}
	}
	return nil
}

func (this *TunnelConnection) write(w io.Writer, data []byte) error {
	start := 0
	for start < len(data) {
		n, err := w.Write(data[start:])
		if err != nil {
			return err
		}
		start += n
		if start < len(data) {
			this.Infof("write next chunk %d", start)
		}
	}
	return nil
}

func (this *TunnelConnection) ReadPacket(data []byte) (int, byte, error) {
	this.rlock.Lock()
	defer this.rlock.Unlock()
	lbuf := [3]byte{}
	err := this.read(this.conn, lbuf[:])

	if err != nil {
		return 0, 0, err
	}

	length := tcp.NtoHs(lbuf[:2])
	if int(length) > len(data) {
		return 0, 0, fmt.Errorf("buffer too small (%d): packet size is %d", len(data), length)
	}
	return int(length), lbuf[2], this.read(this.conn, data[0:length])
}

func (this *TunnelConnection) WritePacket(ty byte, data []byte) error {
	if len(data) > 65535 {
		return fmt.Errorf("packet too large (%d)", len(data))
	}
	lbuf := tcp.HtoNs(uint16(len(data)))
	this.wlock.Lock()
	defer this.wlock.Unlock()
	err := this.write(this.conn, append(lbuf, ty))
	if err != nil {
		return err
	}
	return this.write(this.conn, data)
}

////////////////////////////////////////////////////////////////////////////////

type connectTask struct {
	BaseTask
	name        string
	reconciler  *reconciler
	ratelimiter utils.RateLimiter
}

func NewConnectTask(name string, reconciler *reconciler) Task {
	return &connectTask{
		BaseTask:    NewBaseTask("connect", name),
		name:        name,
		reconciler:  reconciler,
		ratelimiter: utils.NewDefaultRateLimiter(10*time.Second, 10*time.Minute),
	}
}

func (this *connectTask) Execute(logger logger.LogContext) reconcile.Status {
	link := this.reconciler.Links().GetLink(this.name)
	if link == nil {
		logger.Infof("link %s not found", this.name)
		return reconcile.Succeeded(logger)
	}
	_, err := this.reconciler.mux.AssureTunnel(logger, link)
	if err == nil {
		this.ratelimiter.Succeeded()
		return reconcile.Succeeded(logger).RescheduleAfter(10 * time.Minute)
	}
	return reconcile.DelayOnError(logger, err, this.ratelimiter)
}
