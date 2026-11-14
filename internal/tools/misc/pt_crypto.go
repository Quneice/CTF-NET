package misc

// Packet Tracer(.pkt/.pka,经典 ≤7.x)容器解密引擎,纯 Go 零依赖。
// 算法来源:Bugku cisco 复现 writeup + 开源 pka-decipher 系(格式为公开知识):
//  ① stage1 去混淆:整文件反转 + 长度相关 XOR:out[i] = data[L-1-i] ^ byte(L - i*L)
//  ② Twofish-128 魔改 EAX:key=0x89×16、nonce=0x10×16(PT 内嵌固定值);
//     n_tag = CMAC(0x00×16 ‖ nonce)(前缀块直接拼接,非标准 EAX tweak 异或);
//     CTR 自 n_tag 起 128 位大端 +1;密文=stage1[:-16],末 16B 为 tag(解密跳过校验)
//  ③ stage2 去混淆:dec[i] ^ byte(L'-i)
//  ④ 前 4 字节大端=未压缩长度,其后 zlib 流
// Twofish 按 Gladman 风格公开实现移植(表驱动),官方向量见 pt_crypto_test.go。
// 注意:PT 8.x+ 更换加密方案,本引擎不支持(解密失败时由工具层给出提示)。

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/bits"
)

// ---------- Twofish 块密码(128/192/256 位密钥) ----------

var (
	ptTab5B = [4]byte{0, 90, 180, 238}
	ptTabEF = [4]byte{0, 238, 180, 90}
	ptRor4  = [16]byte{0, 8, 1, 9, 2, 10, 3, 11, 4, 12, 5, 13, 6, 14, 7, 15}
	ptAshx  = [16]byte{0, 9, 2, 11, 4, 13, 6, 15, 8, 1, 10, 3, 12, 5, 14, 7}
	ptQt0   = [2][16]byte{{8, 1, 7, 13, 6, 15, 3, 2, 0, 11, 5, 9, 14, 12, 10, 4}, {2, 8, 11, 13, 15, 7, 6, 14, 3, 1, 9, 4, 0, 10, 12, 5}}
	ptQt1   = [2][16]byte{{14, 12, 11, 8, 1, 2, 3, 5, 15, 4, 10, 6, 7, 0, 9, 13}, {1, 14, 2, 11, 4, 12, 3, 7, 6, 13, 10, 5, 15, 9, 0, 8}}
	ptQt2   = [2][16]byte{{11, 10, 5, 14, 6, 13, 9, 0, 12, 8, 15, 3, 2, 4, 7, 1}, {4, 12, 7, 5, 1, 6, 9, 10, 0, 14, 13, 8, 2, 11, 3, 15}}
	ptQt3   = [2][16]byte{{13, 7, 15, 4, 1, 2, 6, 14, 9, 11, 3, 0, 8, 5, 12, 10}, {11, 9, 5, 1, 12, 3, 13, 14, 6, 4, 7, 15, 2, 0, 8, 10}}
)

func ptQP(n int, x byte) byte {
	a0 := x >> 4
	b0 := x & 15
	a1 := a0 ^ b0
	b1 := ptRor4[b0] ^ ptAshx[a0]
	a2 := ptQt0[n][a1]
	b2 := ptQt1[n][b1]
	a3 := a2 ^ b2
	b3 := ptRor4[b2] ^ ptAshx[a2]
	a4 := ptQt2[n][a3]
	b4 := ptQt3[n][b3]
	return b4<<4 | a4
}

func ptByte(x uint32, n int) byte { return byte(x >> (8 * uint(n))) }

type twofish struct {
	kLen  int        // 密钥字数(2/3/4 = 128/192/256 位)
	lKey  [40]uint32 // 轮密钥
	sKey  [4]uint32  // S 盒密钥
	qTab  [2][256]byte
	mTab  [4][256]uint32
	mkTab [4][256]uint32
}

func newTwofish(key []byte) *twofish {
	t := &twofish{}
	if err := t.setKey(key); err != nil {
		panic(err) // 调用方使用固定长度密钥;测试覆盖
	}
	return t
}

func (t *twofish) setKey(key []byte) error {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return fmt.Errorf("twofish 密钥长度须为 16/24/32 字节,实际 %d", len(key))
	}
	for i := 0; i < 256; i++ {
		t.qTab[0][i] = ptQP(0, byte(i))
		t.qTab[1][i] = ptQP(1, byte(i))
	}
	for i := 0; i < 256; i++ {
		f01 := uint32(t.qTab[1][i])
		f5b := f01 ^ (f01 >> 2) ^ uint32(ptTab5B[f01&3])
		fef := f01 ^ (f01 >> 1) ^ (f01 >> 2) ^ uint32(ptTabEF[f01&3])
		t.mTab[0][i] = f01 | f5b<<8 | fef<<16 | fef<<24
		t.mTab[2][i] = f5b | fef<<8 | f01<<16 | fef<<24

		f01 = uint32(t.qTab[0][i])
		f5b = f01 ^ (f01 >> 2) ^ uint32(ptTab5B[f01&3])
		fef = f01 ^ (f01 >> 1) ^ (f01 >> 2) ^ uint32(ptTabEF[f01&3])
		t.mTab[1][i] = fef | fef<<8 | f5b<<16 | f01<<24
		t.mTab[3][i] = f5b | f01<<8 | fef<<16 | f5b<<24
	}
	t.kLen = len(key) * 8 / 64       // 64 位密钥块数(2/3/4)
	kw := make([]uint32, len(key)/4) // 密钥按 32 位小端字展开(16B→4 字)
	for i := range kw {
		kw[i] = binary.LittleEndian.Uint32(key[i*4:])
	}
	var meKey, moKey [4]uint32
	for i := 0; i < t.kLen; i++ {
		a := kw[i*2]
		b := kw[i*2+1]
		meKey[i] = a
		moKey[i] = b
		t.sKey[t.kLen-i-1] = ptMdsRem(a, b)
	}
	for i := 0; i < 40; i += 2 {
		a := uint32(0x01010101 * i)
		b := a + 0x01010101
		a = t.hFun(a, meKey[:])
		b = bits.RotateLeft32(t.hFun(b, moKey[:]), 8)
		t.lKey[i] = a + b
		t.lKey[i+1] = bits.RotateLeft32(a+2*b, 9)
	}
	t.genMkTab()
	return nil
}

func ptMdsRem(p0, p1 uint32) uint32 {
	for i := 0; i < 8; i++ {
		t := p1 >> 24
		p1 = p1<<8 | p0>>24
		p0 = p0 << 8
		u := t << 1
		if t&0x80 != 0 {
			u ^= 0x0000014d
		}
		p1 ^= t ^ (u << 16)
		u ^= t >> 1
		if t&0x01 != 0 {
			u ^= 0x0000014d >> 1
		}
		p1 ^= u<<24 | u<<8
	}
	return p1
}

func (t *twofish) hFun(x uint32, key []uint32) uint32 {
	b0 := ptByte(x, 0)
	b1 := ptByte(x, 1)
	b2 := ptByte(x, 2)
	b3 := ptByte(x, 3)
	if t.kLen >= 4 {
		b0 = t.qTab[1][b0] ^ ptByte(key[3], 0)
		b1 = t.qTab[0][b1] ^ ptByte(key[3], 1)
		b2 = t.qTab[0][b2] ^ ptByte(key[3], 2)
		b3 = t.qTab[1][b3] ^ ptByte(key[3], 3)
	}
	if t.kLen >= 3 {
		b0 = t.qTab[1][b0] ^ ptByte(key[2], 0)
		b1 = t.qTab[1][b1] ^ ptByte(key[2], 1)
		b2 = t.qTab[0][b2] ^ ptByte(key[2], 2)
		b3 = t.qTab[0][b3] ^ ptByte(key[2], 3)
	}
	b0 = t.qTab[0][t.qTab[0][b0]^ptByte(key[1], 0)] ^ ptByte(key[0], 0)
	b1 = t.qTab[0][t.qTab[1][b1]^ptByte(key[1], 1)] ^ ptByte(key[0], 1)
	b2 = t.qTab[1][t.qTab[0][b2]^ptByte(key[1], 2)] ^ ptByte(key[0], 2)
	b3 = t.qTab[1][t.qTab[1][b3]^ptByte(key[1], 3)] ^ ptByte(key[0], 3)
	return t.mTab[0][b0] ^ t.mTab[1][b1] ^ t.mTab[2][b2] ^ t.mTab[3][b3]
}

func (t *twofish) genMkTab() {
	key := t.sKey[:]
	for i := 0; i < 256; i++ {
		by := byte(i)
		switch t.kLen {
		case 2:
			t.mkTab[0][i] = t.mTab[0][t.qTab[0][t.qTab[0][by]^ptByte(key[1], 0)]^ptByte(key[0], 0)]
			t.mkTab[1][i] = t.mTab[1][t.qTab[0][t.qTab[1][by]^ptByte(key[1], 1)]^ptByte(key[0], 1)]
			t.mkTab[2][i] = t.mTab[2][t.qTab[1][t.qTab[0][by]^ptByte(key[1], 2)]^ptByte(key[0], 2)]
			t.mkTab[3][i] = t.mTab[3][t.qTab[1][t.qTab[1][by]^ptByte(key[1], 3)]^ptByte(key[0], 3)]
		case 3:
			t.mkTab[0][i] = t.mTab[0][t.qTab[0][t.qTab[0][t.qTab[1][by]^ptByte(key[2], 0)]^ptByte(key[1], 0)]^ptByte(key[0], 0)]
			t.mkTab[1][i] = t.mTab[1][t.qTab[0][t.qTab[1][t.qTab[1][by]^ptByte(key[2], 1)]^ptByte(key[1], 1)]^ptByte(key[0], 1)]
			t.mkTab[2][i] = t.mTab[2][t.qTab[1][t.qTab[0][t.qTab[0][by]^ptByte(key[2], 2)]^ptByte(key[1], 2)]^ptByte(key[0], 2)]
			t.mkTab[3][i] = t.mTab[3][t.qTab[1][t.qTab[1][t.qTab[0][by]^ptByte(key[2], 3)]^ptByte(key[1], 3)]^ptByte(key[0], 3)]
		case 4:
			t.mkTab[0][i] = t.mTab[0][t.qTab[0][t.qTab[0][t.qTab[1][t.qTab[1][by]^ptByte(key[3], 0)]^ptByte(key[2], 0)]^ptByte(key[1], 0)]^ptByte(key[0], 0)]
			t.mkTab[1][i] = t.mTab[1][t.qTab[0][t.qTab[1][t.qTab[1][t.qTab[0][by]^ptByte(key[3], 1)]^ptByte(key[2], 1)]^ptByte(key[1], 1)]^ptByte(key[0], 1)]
			t.mkTab[2][i] = t.mTab[2][t.qTab[1][t.qTab[0][t.qTab[0][t.qTab[0][by]^ptByte(key[3], 2)]^ptByte(key[2], 2)]^ptByte(key[1], 2)]^ptByte(key[0], 2)]
			t.mkTab[3][i] = t.mTab[3][t.qTab[1][t.qTab[1][t.qTab[0][t.qTab[1][by]^ptByte(key[3], 3)]^ptByte(key[2], 3)]^ptByte(key[1], 3)]^ptByte(key[0], 3)]
		}
	}
}

func (t *twofish) encryptBlock(in []byte) []byte {
	blk := [4]uint32{
		binary.LittleEndian.Uint32(in[0:]) ^ t.lKey[0],
		binary.LittleEndian.Uint32(in[4:]) ^ t.lKey[1],
		binary.LittleEndian.Uint32(in[8:]) ^ t.lKey[2],
		binary.LittleEndian.Uint32(in[12:]) ^ t.lKey[3],
	}
	f := &blk
	for i := 0; i < 8; i++ {
		t1 := t.mkTab[0][ptByte(f[1], 3)] ^ t.mkTab[1][ptByte(f[1], 0)] ^ t.mkTab[2][ptByte(f[1], 1)] ^ t.mkTab[3][ptByte(f[1], 2)]
		t0 := t.mkTab[0][ptByte(f[0], 0)] ^ t.mkTab[1][ptByte(f[0], 1)] ^ t.mkTab[2][ptByte(f[0], 2)] ^ t.mkTab[3][ptByte(f[0], 3)]
		f[2] = bits.RotateLeft32(f[2]^(t0+t1+t.lKey[4*i+8]), -1)
		f[3] = bits.RotateLeft32(f[3], 1) ^ (t0 + 2*t1 + t.lKey[4*i+9])

		t1 = t.mkTab[0][ptByte(f[3], 3)] ^ t.mkTab[1][ptByte(f[3], 0)] ^ t.mkTab[2][ptByte(f[3], 1)] ^ t.mkTab[3][ptByte(f[3], 2)]
		t0 = t.mkTab[0][ptByte(f[2], 0)] ^ t.mkTab[1][ptByte(f[2], 1)] ^ t.mkTab[2][ptByte(f[2], 2)] ^ t.mkTab[3][ptByte(f[2], 3)]
		f[0] = bits.RotateLeft32(f[0]^(t0+t1+t.lKey[4*i+10]), -1)
		f[1] = bits.RotateLeft32(f[1], 1) ^ (t0 + 2*t1 + t.lKey[4*i+11])
	}
	out := make([]byte, 16)
	binary.LittleEndian.PutUint32(out[0:], f[2]^t.lKey[4])
	binary.LittleEndian.PutUint32(out[4:], f[3]^t.lKey[5])
	binary.LittleEndian.PutUint32(out[8:], f[0]^t.lKey[6])
	binary.LittleEndian.PutUint32(out[12:], f[1]^t.lKey[7])
	return out
}

func (t *twofish) decryptBlock(in []byte) []byte {
	f := [4]uint32{
		binary.LittleEndian.Uint32(in[0:]) ^ t.lKey[4],
		binary.LittleEndian.Uint32(in[4:]) ^ t.lKey[5],
		binary.LittleEndian.Uint32(in[8:]) ^ t.lKey[6],
		binary.LittleEndian.Uint32(in[12:]) ^ t.lKey[7],
	}
	for i := 7; i >= 0; i-- {
		t1 := t.mkTab[0][ptByte(f[1], 3)] ^ t.mkTab[1][ptByte(f[1], 0)] ^ t.mkTab[2][ptByte(f[1], 1)] ^ t.mkTab[3][ptByte(f[1], 2)]
		t0 := t.mkTab[0][ptByte(f[0], 0)] ^ t.mkTab[1][ptByte(f[0], 1)] ^ t.mkTab[2][ptByte(f[0], 2)] ^ t.mkTab[3][ptByte(f[0], 3)]
		f[2] = bits.RotateLeft32(f[2], 1) ^ (t0 + t1 + t.lKey[4*i+10])
		f[3] = bits.RotateLeft32(f[3]^(t0+2*t1+t.lKey[4*i+11]), -1)

		t1 = t.mkTab[0][ptByte(f[3], 3)] ^ t.mkTab[1][ptByte(f[3], 0)] ^ t.mkTab[2][ptByte(f[3], 1)] ^ t.mkTab[3][ptByte(f[3], 2)]
		t0 = t.mkTab[0][ptByte(f[2], 0)] ^ t.mkTab[1][ptByte(f[2], 1)] ^ t.mkTab[2][ptByte(f[2], 2)] ^ t.mkTab[3][ptByte(f[2], 3)]
		f[0] = bits.RotateLeft32(f[0], 1) ^ (t0 + t1 + t.lKey[4*i+8])
		f[1] = bits.RotateLeft32(f[1]^(t0+2*t1+t.lKey[4*i+9]), -1)
	}
	out := make([]byte, 16)
	binary.LittleEndian.PutUint32(out[0:], f[2]^t.lKey[0])
	binary.LittleEndian.PutUint32(out[4:], f[3]^t.lKey[1])
	binary.LittleEndian.PutUint32(out[8:], f[0]^t.lKey[2])
	binary.LittleEndian.PutUint32(out[12:], f[1]^t.lKey[3])
	return out
}

// ---------- CMAC(OMAC1) / CTR ----------

func ptXor16(a, b []byte) []byte {
	out := make([]byte, 16)
	for i := 0; i < 16; i++ {
		out[i] = a[i] ^ b[i]
	}
	return out
}

func ptDbl(b []byte) []byte {
	hi := binary.BigEndian.Uint64(b[:8])
	lo := binary.BigEndian.Uint64(b[8:])
	carry := lo >> 63  // 低位组 MSB 进高位组 LSB
	hi = hi<<1 | carry // 高位组原 MSB 被移出,由 0x87 归约处理
	lo = lo << 1
	out := make([]byte, 16)
	binary.BigEndian.PutUint64(out[:8], hi)
	binary.BigEndian.PutUint64(out[8:], lo)
	if b[0]&0x80 != 0 {
		out[15] ^= 0x87
	}
	return out
}

// ptCMAC 标准OMAC1(128 位分组)。
func ptCMAC(tf *twofish, msg []byte) []byte {
	L := tf.encryptBlock(make([]byte, 16))
	k1 := ptDbl(L)
	k2 := ptDbl(k1)
	var blocks [][]byte
	var last []byte
	if len(msg) == 0 {
		padded := append([]byte{0x80}, make([]byte, 15)...)
		last = ptXor16(padded, k2)
	} else {
		for i := 0; i < len(msg); i += 16 {
			blocks = append(blocks, msg[i:i+16])
		}
		if len(blocks[len(blocks)-1]) == 16 {
			last = ptXor16(blocks[len(blocks)-1], k1)
			blocks = blocks[:len(blocks)-1]
		} else {
			l := blocks[len(blocks)-1]
			padded := append(append([]byte{}, l...), 0x80)
			for len(padded) < 16 {
				padded = append(padded, 0)
			}
			last = ptXor16(padded, k2)
			blocks = blocks[:len(blocks)-1]
		}
	}
	x := make([]byte, 16)
	for _, b := range blocks {
		x = tf.encryptBlock(ptXor16(x, b))
	}
	return tf.encryptBlock(ptXor16(x, last))
}

// ptCTR 自 ctr0 起 128 位大端 +1 的计数器模式。
func ptCTR(tf *twofish, ctr0 []byte, data []byte) []byte {
	out := make([]byte, len(data))
	hi := binary.BigEndian.Uint64(ctr0[:8])
	lo := binary.BigEndian.Uint64(ctr0[8:])
	buf := make([]byte, 16)
	for i := 0; i < len(data); i += 16 {
		binary.BigEndian.PutUint64(buf[:8], hi)
		binary.BigEndian.PutUint64(buf[8:], lo)
		ks := tf.encryptBlock(buf)
		end := i + 16
		if end > len(data) {
			end = len(data)
		}
		for j := i; j < end; j++ {
			out[j] = data[j] ^ ks[j-i]
		}
		lo++
		if lo == 0 {
			hi++
		}
	}
	return out
}

// ---------- PT 容器 ----------

var (
	ptKey   = bytes.Repeat([]byte{0x89}, 16)
	ptNonce = bytes.Repeat([]byte{0x10}, 16)
)

// ptNtag 魔改 EAX 的初始计数器:N = CMAC(0x00×16 ‖ nonce)(前缀块直接拼接,非标准 tweak)。
func ptNtag(tf *twofish) []byte {
	return ptCMAC(tf, append(make([]byte, 16), ptNonce...))
}

// decryptPT 解密 Packet Tracer 容器(≤7.x 固定密钥),返回明文(XML 等)。
func decryptPT(data []byte) ([]byte, error) {
	if len(data) < 22+16 {
		return nil, errors.New("文件过小,不像 Packet Tracer 容器")
	}
	L := len(data)
	stage1 := make([]byte, L)
	for i := 0; i < L; i++ {
		stage1[i] = data[L-1-i] ^ byte(L-i*L) // 有符号低字节截断,与 Python &0xFF 等价
	}
	tf := newTwofish(ptKey)
	dec := ptCTR(tf, ptNtag(tf), stage1[:L-16]) // 末 16B 为 tag,跳过校验
	L2 := len(dec)
	stage2 := make([]byte, L2)
	for i, b := range dec {
		stage2[i] = b ^ byte(L2-i)
	}
	if len(stage2) < 5 {
		return nil, errors.New("解密后数据异常(长度不足)")
	}
	size := binary.BigEndian.Uint32(stage2[:4])
	zr, err := zlib.NewReader(bytes.NewReader(stage2[4:]))
	if err != nil {
		return nil, errors.New("zlib 头校验失败:可能不是 Packet Tracer 容器,或为 PT 8.x+ 新加密格式(本工具支持 ≤7.x)")
	}
	// 解压上限 512MB:真实 PT 工程约 1MB;防伪造容器的 zlib 炸弹撑爆内存(仓库惯例同 image_meta)
	const maxXML = 512 << 20
	raw, err := io.ReadAll(io.LimitReader(zr, maxXML+1))
	if err != nil {
		return nil, fmt.Errorf("zlib 解压失败: %v(可能不是 PT 容器)", err)
	}
	if len(raw) > maxXML {
		return nil, errors.New("解压结果超过 512MB 上限:疑似伪造容器(zlib 炸弹)")
	}
	if size > 0 && uint64(size) <= uint64(len(raw)) { // uint64 比较,防 32 位平台 int 溢出(终审#3)
		raw = raw[:size]
	}
	return raw, nil
}

// encryptPT decryptPT 的逆函数(测试合成样本用;tag 字节无意义,解密跳过)。
func encryptPT(plain []byte) []byte {
	var zb bytes.Buffer
	zw := zlib.NewWriter(&zb)
	_, _ = zw.Write(plain)
	_ = zw.Close()
	blob := make([]byte, 4, 4+zb.Len())
	binary.BigEndian.PutUint32(blob, uint32(len(plain)))
	blob = append(blob, zb.Bytes()...)
	L2 := len(blob)
	dec := make([]byte, L2)
	for i, b := range blob {
		dec[i] = b ^ byte(L2-i)
	}
	tf := newTwofish(ptKey)
	ct := ptCTR(tf, ptNtag(tf), dec)
	stage1 := append(append([]byte{}, ct...), make([]byte, 16)...) // 16B 占位 tag
	L := len(stage1)
	// 加密 = 解密的逆:先按下标 XOR,再整体反转(解密公式非对合,不能套两遍)
	out := make([]byte, L)
	for i := 0; i < L; i++ {
		out[i] = stage1[i] ^ byte(L-i*L)
	}
	for i, j := 0, L-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
