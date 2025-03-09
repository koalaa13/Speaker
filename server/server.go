package main

import (
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"log"
	"net"
	"proto"
	"types"
)

type server struct {
	proto.UnimplementedAudioServiceServer
	broadcastAudioCaches map[string]*types.AudioCache
	registeredClients    map[string]grpc.BidiStreamingServer[proto.AudioInfo, proto.AudioInfo]
}

func (s *server) Connect(stream grpc.BidiStreamingServer[proto.AudioInfo, proto.AudioInfo]) error {
	log.Println("new stream connection established")
	ctx := stream.Context()
	md, _ := metadata.FromIncomingContext(ctx)
	clientIds, has := md[types.ClientIdKey]
	if !has {
		log.Println("there is no client id")
		return errors.New("there is no client id")
	}
	clientId := clientIds[0]
	if s.registeredClients[clientId] == nil {
		s.registeredClients = make(map[string]grpc.BidiStreamingServer[proto.AudioInfo, proto.AudioInfo])
	}
	s.registeredClients[clientId] = stream
	if s.broadcastAudioCaches == nil {
		s.broadcastAudioCaches = make(map[string]*types.AudioCache)
	}
	s.broadcastAudioCaches[clientId] = types.NewAudioCache()

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("stream connection closed: " + ctx.Err().Error())
				return
			default:
			}

			toSend, hasData := s.broadcastAudioCaches[clientId].Read()
			if !hasData {
				continue
			}
			for cId, clientStream := range s.registeredClients {
				if cId != clientId {
					audioInfo := proto.AudioInfo{
						ClientId: clientId,
						Samples:  toSend,
					}
					if err := clientStream.Send(&audioInfo); err != nil {
						log.Println("failed to send audio info: " + err.Error())
					}
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("stream connection closed: " + ctx.Err().Error())
				return
			default:
			}

			audio, err := stream.Recv()
			if err != nil {
				log.Println("stream connection closed: " + ctx.Err().Error())
				break
			}

			if audio != nil {
				log.Println("received audio: " + audio.String())
				s.broadcastAudioCaches[clientId].Write(audio.GetSamples())
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Println("stream closed: " + ctx.Err().Error())
			delete(s.broadcastAudioCaches, clientId)
			delete(s.registeredClients, clientId)
			return nil
		default:
		}
	}
}

func main() {
	l, err := net.Listen("tcp", ":6006")
	if err != nil {
		panic(err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterAudioServiceServer(grpcServer, &server{})
	if err = grpcServer.Serve(l); err != nil {
		panic(err)
	}
}
