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
	"strconv"
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
	sdb     sync.Map
	vsDb    sync.Map
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
		for _, data := range req.Nodes {
			s.sdb.Store(string(data.Addr), data.Node)
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

	//for客户端
	for _, sst := range sts {
		//客户端游标
		st := sst
		//for查询轮
		for {
			tt := slices.Concat(tw, st)
			ed, ok := s.sdb.Load(string(utils.H1(string(tt))))
			if !ok {
				//log.Printf("sts[%v] not found", st[:8])
				break
			}
			sk := utils.Xor(ed.([]byte), utils.H2(string(tt)))
			ost := sk[:16]
			kw := sk[16:]

			cnt := 1
			//for数据链
			for {
				ut := string(utils.H3(string(kw) + strconv.Itoa(cnt)))
				eid, ok1 := s.db.Load(ut)
				if !ok1 {
					break
					//log.Printf("sts[%v] not found", st[:8])
				}
				vs, ok2 := s.vsDb.Load(ut)
				if !ok2 {
					return errors.New("not found version")
				}
				vv := vs.(*int64)
				//for+CAS
				for {
					v := atomic.LoadInt64(vv)
					if v > 0 {
						if v < svs {
							_ = stream.Send(&cemm.GetReply{Node: eid.([]byte)})
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
				cnt++ //数据链游标
			}
			st = slices.Clone(ost) //状态链游标

		}

	}

	return nil
}

func (s *EMMServer) Add(ctx context.Context, in *cemm.AddRequest) (*emptypb.Empty, error) {
	var v int64
	s.vsDb.Store(string(in.NewNode.Addr), &v)
	s.db.Store(string(in.NewNode.Addr), in.NewNode.Node)
	if in.SrchNode != nil {
		s.sdb.Store(string(in.SrchNode.Addr), in.SrchNode.Node)
	}

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
