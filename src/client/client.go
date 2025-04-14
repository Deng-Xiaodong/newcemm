package client

import (
	"DRW/src/config"
	"DRW/src/rpc/cemm"
	"DRW/src/utils"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"slices"
	"strconv"
	"time"
)

var AESKEY = []byte("abcdefg123456789")
var PRFKEY = []byte("123456789abcdefg")
var DUMMY []byte

type EMMClient struct {
	cnt, volume, limitRound int
	idx                     int
	state                   map[string]*roundCount
	//方案暂时假定后续添加不能超过初始化的关键字空间
	stub cemm.CEMMClient
}
type roundCount struct {
	round int
	count int
}

func NewEMMClient(idx int, cf *config.Config, stub cemm.CEMMClient) *EMMClient {
	var err error
	DUMMY, err = utils.AESEncryptCBC(AESKEY, []byte("dummy"))
	if err != nil {
		log.Fatal(err)
	}
	return &EMMClient{
		cnt:        cf.ClientCnt,
		volume:     cf.Volume,
		limitRound: cf.LimitRound,
		idx:        idx,
		state:      make(map[string]*roundCount),
		stub:       stub,
	}
}

func (c *EMMClient) getClientRoundStart(round int) int {
	return c.cnt*c.volume*(round-1) + c.volume*(c.idx-1) + 1
}
func (c *EMMClient) getClientRoundEnd(round int) int {
	return c.cnt*c.volume*(round-1) + c.volume*c.idx
}
func (c *EMMClient) getClientRoundEndWithId(round, id int) int {
	return c.cnt*c.volume*(round-1) + c.volume*id
}
func (c *EMMClient) getAllClientRoundEnd(round int) (sts []int) {
	for i := 1; i <= c.cnt; i++ {
		sts = append(sts, c.getClientRoundEndWithId(round, i))
	}
	return
}
func (c *EMMClient) getRoundEnd(round int) int {
	return c.cnt * c.volume * round
}

func (c *EMMClient) genAddToken(keyword, value string, round int, hw []byte) (next, preNext *cemm.AddToken, err error) {

	cipherValue, err := utils.AESEncryptCBC(AESKEY, []byte(value))
	if err != nil {
		return nil, nil, err
	}
	//获取关键字计数
	rc := c.state[keyword]
	if rc == nil {
		rc = &roundCount{}
		c.state[keyword] = rc
	}
	if rc.round < round {
		rc.round = round
		rc.count = c.getClientRoundStart(rc.round)
	}
	cnt := rc.count
	rc.count++

	oldSt := genSt(hw, cnt-1)
	newSt := genSt(hw, cnt)
	endSt := genSt(hw, c.getClientRoundEnd(round))

	zero := make([]byte, 4, 4)
	binary.BigEndian.PutUint32(zero, 0)

	//生成字典键值对
	//tag := genKwTag(hw)
	//node := slices.Concat(zero, utils.Xor(slices.Concat(oldSt, cipherValue), utils.H2(string(slices.Concat(tag, newSt)))))
	//endNode := slices.Concat(zero, utils.Xor(slices.Concat(newSt, DUMMY), utils.H2(string(slices.Concat(tag, endSt)))))
	node := slices.Concat(zero, oldSt, cipherValue)
	endNode := slices.Concat(zero, newSt, DUMMY)

	//todo 满了，增加轮数

	//return &cemm.AddToken{Addr: utils.H1(string(slices.Concat(tag, newSt))), Node: node},
	//	&cemm.AddToken{Addr: utils.H1(string(slices.Concat(tag, endSt))), Node: endNode}, nil
	return &cemm.AddToken{Addr: newSt, Node: node}, &cemm.AddToken{Addr: endSt, Node: endNode}, nil

}

func (c *EMMClient) genGetToken(hw []byte, round int) *cemm.GetRequest {
	return &cemm.GetRequest{Tag: genKwTag(hw), St: genSt(hw, c.getRoundEnd(round))}
}

func genKwTag(hw []byte) []byte {
	return utils.PRF(PRFKEY, hw)
}
func genKwHash(keyword string) []byte {
	return utils.H1(keyword)
}
func genSt(hw []byte, i int) []byte {
	return utils.PRF(PRFKEY, hw, i)
}

func (c *EMMClient) Add(keyword, value string) error {

	hw := genKwHash(keyword)
	var round int
	if rly, err := c.stub.GetOrIncRound(context.Background(), &cemm.RoundRequest{Op: false, Hw: hw}); err != nil {
		return err
	} else {
		round = int(rly.Round)
	}

	next, preNext, err := c.genAddToken(keyword, value, round, hw)
	if err != nil {
		return err
	}
	if _, err = c.stub.Add(context.Background(), &cemm.AddRequest{Next: next, PreNext: preNext}); err != nil {
		return err
	}
	return nil
}

func (c *EMMClient) Get(keyword string) ([]string, error) {
	hw := genKwHash(keyword)

	var round int
	if rly, err := c.stub.GetOrIncRound(context.Background(), &cemm.RoundRequest{Op: true, Hw: hw}); err != nil {
		log.Printf("RPC ERROR: GetRound fail %v\n", err)
		return nil, err
	} else {
		round = int(rly.Round)
	}

	gtk := c.genGetToken(hw, round)
	var res []string

	//测试用
	res = append(res, strconv.Itoa(round))

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
			var text []byte
			text, err = utils.AESDecryptCBC(AESKEY, recv.Node)
			if err != nil {
				return nil, err
			}
			res = append(res, string(text))
		}

	} else {
		return nil, err
	}
	return res, nil
}
func (c *EMMClient) Init(data [][]string) error {

	stream, err := c.stub.Init(context.Background())
	if err != nil {
		return err
	}
	for _, cell := range data {
		initTokens := make([]*cemm.AddToken, 0, len(cell)+c.cnt*c.limitRound+2)
		hw := genKwHash(cell[0]) //RPF的输入，生成st
		//tag := genKwTag(hw)      //哈希函数的输入，保护数据
		one := make([]byte, 4, 4)
		zero := make([]byte, 4, 4)
		binary.BigEndian.PutUint32(one, 1)
		binary.BigEndian.PutUint32(zero, 0)
		//先处理数据在处理dummy
		orst := make([]byte, 32, 32)
		nrst := make([]byte, 32, 32)
		var rnode []byte
		io.ReadFull(rand.Reader, orst)
		var cx []byte
		cx, err = utils.AESEncryptCBC(AESKEY, []byte(cell[1]))
		if err != nil {
			return err
		}
		//rnode = slices.Concat(zero, utils.Xor(slices.Concat(slices.Repeat([]byte{'0'}, 32), cx), utils.H2(string(slices.Concat(tag, orst)))))
		//initTokens = append(initTokens, &cemm.AddToken{Addr: utils.H1(string(slices.Concat(tag, orst))), Node: slices.Clone(rnode)})
		rnode = slices.Concat(one, slices.Repeat([]byte{'0'}, 32), cx)
		initTokens = append(initTokens, &cemm.AddToken{Addr: slices.Clone(orst), Node: slices.Clone(rnode)})
		for i := 2; i < len(cell); i++ {
			io.ReadFull(rand.Reader, nrst)
			cx, err = utils.AESEncryptCBC(AESKEY, []byte(cell[i]))
			if err != nil {
				return err
			}
			//rnode = slices.Concat(zero, utils.Xor(slices.Concat(orst, cx), utils.H2(string(slices.Concat(tag, nrst)))))
			//initTokens = append(initTokens, &cemm.AddToken{
			//	Addr: utils.H1(string(slices.Concat(tag, nrst))),
			//	Node: slices.Clone(rnode),
			//})
			rnode = slices.Concat(one, orst, cx)
			initTokens = append(initTokens, &cemm.AddToken{
				Addr: slices.Clone(nrst),
				Node: slices.Clone(rnode),
			})
			orst = slices.Clone(nrst) //第一行会改变
		}

		if cell[0] != "key1" {
			continue
		}

		odst := genSt(hw, 0)
		var ndst []byte
		//dnode := slices.Concat(zero, utils.Xor(slices.Concat(orst, DUMMY), utils.H2(string(slices.Concat(tag, odst)))))
		//initTokens = append(initTokens, &cemm.AddToken{Addr: utils.H1(string(slices.Concat(tag, odst))), Node: slices.Clone(dnode)})
		dnode := slices.Concat(zero, orst, DUMMY)
		initTokens = append(initTokens, &cemm.AddToken{
			Addr: slices.Clone(odst),
			Node: slices.Clone(dnode),
		})

		t := c.volume
		for i := 1; i <= c.limitRound; i++ {
			for j := 1; j <= c.cnt; j++ {
				ndst = genSt(hw, t)
				t += c.volume
				//dnode = slices.Concat(zero, utils.Xor(slices.Concat(odst, DUMMY), utils.H2(string(slices.Concat(tag, ndst)))))
				//initTokens = append(initTokens, &cemm.AddToken{Addr: utils.H1(string(slices.Concat(tag, ndst))), Node: slices.Clone(dnode)})
				dnode = slices.Concat(zero, odst, DUMMY)
				initTokens = append(initTokens, &cemm.AddToken{
					Addr: slices.Clone(ndst),
					Node: slices.Clone(dnode),
				})
				//odst = ndst //第一行不会改变，最好还是用副本
				odst = slices.Clone(ndst)
			}

		}
		//初始化一个关键字
		if err = stream.Send(&cemm.InitRequest{Hw: hw, Nodes: initTokens}); err != nil {
			return err
		}

	}
	//关闭流
	time.Sleep(time.Second)
	_ = stream.CloseSend()

	return nil

}
