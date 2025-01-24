package main

import (
	"google.golang.org/grpc"
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
	clientId := ctx.Value("clientId").(string)
	s.registeredClients[clientId] = stream
	s.broadcastAudioCaches[clientId] = &types.AudioCache{}

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("stream connection closed: " + ctx.Err().Error())
				return
			default:
			}

			toSend := s.broadcastAudioCaches[clientId].Read()
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
