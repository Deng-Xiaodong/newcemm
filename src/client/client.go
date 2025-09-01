package client

import (
	"DRW/src/rpc/cemm"
	"DRW/src/utils"
	"context"
	"errors"
	"fmt"
	"google.golang.org/protobuf/types/known/emptypb"
	"io"
	"log"
	"slices"
	"strconv"
	"time"
)

const (
	LIMITROUND = 10000
	N          = 8
)

var (
	AESKEY  = []byte("abcdefg123456789")
	PRFKEY  = []byte("123456789abcdefg")
	DummyId []byte
)

type EMMClient struct {
	uid   int
	state map[string]*roundCount
	//方案暂时假定后续添加不能超过初始化的关键字空间
	stub cemm.CEMMClient
}
type roundCount struct {
	round int
	cnt   int
}

func NewEMMClient(uid int, stub cemm.CEMMClient) *EMMClient {
	did, err := utils.AESEncryptCBC(AESKEY, []byte("dummyId"))
	if err != nil {
		log.Fatal(err)
	}
	DummyId = did
	return &EMMClient{
		uid:   uid,
		state: make(map[string]*roundCount),
		stub:  stub,
	}
}

func (c *EMMClient) genAddToken(w, id string, round int, tw []byte) (newNode, srchNode *cemm.AddToken, err error) {

	eid, err := utils.AESEncryptCBC(AESKEY, []byte(id))
	if err != nil {
		return nil, nil, err
	}
	//获取关键字计数
	rc := c.state[w]
	if rc == nil {
		rc = &roundCount{round: 1, cnt: 1}
		c.state[w] = rc
	}
	kw := genKw(w, c.uid, round)
	if rc.round < round {
		ost := genSt(w, c.uid, rc.round)
		nst := genSt(w, c.uid, round)
		tt := slices.Concat(tw, nst)
		sk := slices.Concat(ost, kw)
		srchNode = &cemm.AddToken{
			Addr: utils.H1(string(tt)),
			Node: utils.Xor(sk, utils.H2(string(tt))),
		}
		rc.round = round
		rc.cnt = 1
	}
	cnt := rc.cnt
	rc.cnt++
	newNode = &cemm.AddToken{
		Addr: utils.H3(string(kw) + strconv.Itoa(cnt)),
		Node: eid,
	}
	return

}

func (c *EMMClient) genGetToken(tw []byte, w string, round int, vs int64) *cemm.GetRequest {
	sts := make([][]byte, 0, N)
	for i := 1; i <= N; i++ {
		sts = append(sts, genSt(w, i, round))
	}
	return &cemm.GetRequest{Tw: tw, Sts: sts, Vs: vs}
}

func genTw(w string) []byte {
	return utils.PRF(PRFKEY, utils.H0(w))
}
func genSt(w string, uid, r int) []byte {
	return utils.PRF(PRFKEY, append(utils.H0(w), []byte(fmt.Sprintf("%d%d%d", uid, r, 0))...))[:16]
}
func genKw(w string, uid, r int) []byte {
	return utils.PRF(PRFKEY, append(utils.H0(w), []byte(fmt.Sprintf("%d%d%d", uid, r, 1))...))[:16]
}

func (c *EMMClient) Add(w, id string) error {

	tw := genTw(w)
	var round int
	if rly, err := c.stub.AddRound(context.Background(), &emptypb.Empty{}); err != nil {
		return err
	} else {
		round = int(rly.Round)
	}

	newNode, srchNode, err := c.genAddToken(w, id, round, tw)
	if err != nil {
		return err
	}
	if _, err = c.stub.Add(context.Background(), &cemm.AddRequest{NewNode: newNode, SrchNode: srchNode}); err != nil {
		return err
	}
	return nil
}
func (c *EMMClient) Get(w string) ([]string, error) {
	tw := genTw(w)

	var round int
	var vs int64
	if rly, err := c.stub.SearchRound(context.Background(), &emptypb.Empty{}); err != nil {
		log.Printf("RPC ERROR: GetRound fail %v\n", err)
		return nil, err
	} else {
		round = int(rly.Round)
		vs = rly.Vs
	}
	//log.Printf("Get round %d, vs %d\n", round, vs)
	gtk := c.genGetToken(tw, w, round, vs)
	var res []string

	//测试用
	res = append(res, strconv.Itoa(round))
	var cps [][]byte
	if stream, err := c.stub.Get(context.Background(), gtk); err == nil {
		for {
			recv, errRecv := stream.Recv()
			if errRecv != nil {
				if errors.Is(errRecv, io.EOF) {
					break
				}
				log.Printf("RPC ERROR: Get fail %v\n", errRecv)
				return nil, errRecv
			}
			cps = append(cps, recv.Node)
		}

	} else {
		return nil, err
	}

	for _, cp := range cps {
		var text []byte
		text, err := utils.AESDecryptCBC(AESKEY, cp)
		if err != nil {
			return nil, err
		}
		if string(text) != "dummyId" {
			res = append(res, string(text))
		}

	}

	return res, nil
}
func (c *EMMClient) Init(ws []string) error {

	stream, err := c.stub.Init(context.Background())
	if err != nil {
		return err
	}
	for uid := 1; uid <= N; uid++ {
		for _, w := range ws {
			initTokens := make([]*cemm.AddToken, 0, LIMITROUND)
			tw := genTw(w)

			ost := genSt(w, uid, 0)
			var nst []byte

			for r := 1; r <= LIMITROUND; r++ {
				nst = genSt(w, uid, r)
				kw := genKw(w, uid, r)
				tt := slices.Concat(tw, nst)
				initTokens = append(initTokens, &cemm.AddToken{
					Addr: utils.H1(string(tt)),
					Node: utils.Xor(slices.Concat(ost, kw), utils.H2(string(tt))),
				})
				ost = slices.Clone(nst)
			}
			//初始化一个关键字
			if err = stream.Send(&cemm.InitRequest{Nodes: initTokens}); err != nil {
				return err
			}

		}
	}

	//关闭流
	time.Sleep(time.Second)
	_ = stream.CloseSend()

	return nil

}
