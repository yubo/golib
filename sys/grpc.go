package sys

import (
	"net"

	"github.com/yubo/golib/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"k8s.io/klog/v2"
)

func GrpcRegster(sd *grpc.ServiceDesc, srv interface{}) {
	_module.grpcServer.RegisterService(sd, srv)
}

func (p *Module) grpcPrestart() {
	var opt []grpc.ServerOption

	if p.GrpcMaxRecvMsgSize > 0 {
		klog.V(5).Infof("set grpc server max recv msg size %s",
			util.ByteSize(p.GrpcMaxRecvMsgSize).HumanReadable())
		opt = append(opt, grpc.MaxRecvMsgSize(p.GrpcMaxRecvMsgSize))
	}

	p.grpcServer = grpc.NewServer(opt...)
}

func (p *Module) grpcStart() error {

	if util.AddrIsDisable(p.GrpcAddr) {
		return nil
	}

	ln, err := net.Listen(util.CleanSockFile(util.ParseAddr(p.GrpcAddr)))
	if err != nil {
		return err
	}
	klog.V(5).Infof("ListenAndServe addr %s", p.GrpcAddr)

	reflection.Register(p.grpcServer)

	go func() {
		if err := p.grpcServer.Serve(ln); err != nil {
			return
		}
	}()

	go func() {
		<-p.ctx.Done()
		p.grpcServer.GracefulStop()
	}()

	return nil
}
