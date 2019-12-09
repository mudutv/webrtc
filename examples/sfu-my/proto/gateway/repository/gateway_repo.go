package repository

import (
	"context"
	"fmt"
	"git.mudu.tv/myun/utils"
	"git.mudu.tv/streaming/mrtc-gateway/proto/gateway"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"time"
)

type GatewaySvcRepo interface {
	CloseStream(streamKey string) error
}

const configGRPCAddrEnvKey = "GATEWAY_SERVICE_GRPC_ADDR"
const reqTimeout = 10 * time.Second
const errorCodeOK = 1000

func NewConfigSvcRepository() GatewaySvcRepo {
	storageServiceAddr := utils.GetEnvWithDefault(configGRPCAddrEnvKey, "mrtc-gateway.default:8001")
	conn, err := grpc.Dial(storageServiceAddr, grpc.WithInsecure(),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 10 * time.Second, Timeout: 10 * time.Second, PermitWithoutStream: true}))
	if err != nil {
		log.WithError(err).Fatalln("连接config微服务失败")
	}

	client := gateway.NewRtcInterfaceClient(conn)
	return &gatewaySvcRepo{client}
}

type gatewaySvcRepo struct {
	client gateway.RtcInterfaceClient
}

func (r gatewaySvcRepo) CloseStream(streamKey string) error {
	log.Infof("[gateway:repo.CloseStream] 准备关闭流: streamKey=%s", streamKey)

	ctx, cancel := context.WithTimeout(context.Background(), reqTimeout)
	defer cancel()
	req := &gateway.StreamInfo{
		Key: streamKey,
	}
	resp, err := r.client.CloseStream(ctx, req)
	if resp != nil {
		err = adapterError(resp.ErrCode, err)
	}
	if err != nil {
		log.WithError(err).Errorf("[gateway:repo.CloseStream] 关闭流失败: streamKey=%s", streamKey)
		return err
	}

	log.Infof("[gateway:repo.CloseStream] 关闭流成功: streamKey=%s", streamKey)
	return nil
}

func adapterError(errCode uint64, err error) error {
	if errCode != errorCodeOK {
		return fmt.Errorf("gateway: gRPC 调用返回错误码: %d", errCode)
	} else {
		return err
	}
}
