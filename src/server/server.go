package server

import (
	"DRW/src/rpc/cemm"
	"DRW/src/utils"
	"context"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"io"
	"log"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

type AtomicCounter struct {
	value int64
}

func NewAtomicCounter(v int64) *AtomicCounter {
	return &AtomicCounter{v}
}
func (ac *AtomicCounter) Load() int64 {
	return atomic.LoadInt64(&ac.value)
}
func (ac *AtomicCounter) CAS(old, new int64) bool {
	return atomic.CompareAndSwapInt64(&ac.value, old, new)
}
func (ac *AtomicCounter) FetchAndInc() int64 {
	for {
		old := atomic.LoadInt64(&ac.value)
		if atomic.CompareAndSwapInt64(&ac.value, old, old+1) {
			return old
		}
	}
}

type EMMServer struct {
	cemm.UnimplementedCEMMServer

	version *AtomicCounter
	round   *AtomicCounter
	db      sync.Map
	vsDb    sync.Map

	//workChan chan int64
	//tc       int
}

func (s *EMMServer) AddRound(ctx context.Context, empty *emptypb.Empty) (*cemm.AddRoundReply, error) {
	return &cemm.AddRoundReply{
		Round: s.round.Load(),
	}, nil
}

func (s *EMMServer) SearchRound(ctx context.Context, empty *emptypb.Empty) (*cemm.SearchRoundReply, error) {
	var r, vs int64
	for {
		r = s.round.Load()
		vs = s.version.FetchAndInc()
		if s.round.CAS(r, r+1) {
			break
		}
	}
	return &cemm.SearchRoundReply{
		Round: r,
		Vs:    vs,
	}, nil
}

func (s *EMMServer) Init(stream grpc.ClientStreamingServer[cemm.InitRequest, emptypb.Empty]) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		var v int64 = 1
		for _, data := range req.Nodes {
			s.vsDb.Store(string(data.Addr), &v)
			s.db.Store(string(data.Addr), data.Node)
		}

	}
}

func NewEMMServer() *EMMServer {
	s := &EMMServer{
		version: NewAtomicCounter(2), //初始DB和查询结点的版本号为1
		round:   NewAtomicCounter(1),
	}
	return s
}

func (s *EMMServer) Get(in *cemm.GetRequest, stream grpc.ServerStreamingServer[cemm.GetReply]) error {

	sts := in.Sts
	tw := in.Tw
	svs := in.Vs

	for _, sst := range sts {
		st := sst
		for {
			ut := utils.H1(string(slices.Concat(tw, st)))
			value, ok1 := s.db.Load(string(ut))
			if !ok1 {
				break
			}
			data := value.([]byte)
			cid := data[:32]
			st = utils.Xor(data[32:], utils.H2(string(slices.Concat(tw, st))))

			vs, ok2 := s.vsDb.Load(string(ut))
			if !ok2 {
				return errors.New("not found version")
			}
			vv := vs.(*int64)
			for {
				v := atomic.LoadInt64(vv)
				if v > 0 {
					if v < svs {
						_ = stream.Send(&cemm.GetReply{Node: cid})
					} else {
						log.Printf("miss node svs(%d)<nv(%d)\n", svs, v)
					}
					break
				} else {
					if atomic.CompareAndSwapInt64(vv, v, -svs) {
						break
					}
				}

			}
		}
	}

	return nil
}

func (s *EMMServer) Add(ctx context.Context, in *cemm.AddRequest) (*emptypb.Empty, error) {
	var v int64
	s.vsDb.Store(string(in.NewNode.Addr), &v)
	s.db.Store(string(in.NewNode.Addr), in.NewNode.Node)
	s.db.Store(string(in.SrchNode.Addr), in.SrchNode.Node)

	time.Sleep(6 * time.Millisecond) //测试用
	for {
		val := atomic.LoadInt64(&v)
		vs := s.version.FetchAndInc()
		if atomic.CompareAndSwapInt64(&v, val, vs) {
			//log.Printf("add node version: %d\n", vs)
			break
		}
	}

	return &emptypb.Empty{}, nil
}
