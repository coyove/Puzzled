package main

import (
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"math/rand"
	"os"
	"time"
)

func generateList(length, max int, seed uint64) []int {
	ret := make([]int, length)
	i := 0
	for i < length {
		seed = (2097151*seed + 13739) % 4294967296
		ret[i] = int((float64(seed) / 4294967296) * float64(max))
		i++
	}

	return ret
}

func puzzle(fn string, out string, pass uint64) error {
	f, err := os.Open(fn)
	if err != nil {
		return err
	}

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	rect := img.Bounds()
	size := rect.Size()
	width, height := size.X, size.Y

	final := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(final, rect, img, rect.Min, draw.Src)

	block := 32
	wBlocks := width / block
	hBlocks := height / block

	_ = rand.New(rand.NewSource(time.Now().UnixNano()))

	exBuf := make([]byte, block*4)
	exBlock := func(pos1, pos2 int) {
		addr1, addr2 := pos1*4, pos2*4

		for i := 0; i < block; i++ {
			copy(exBuf[:], final.Pix[addr1:addr1+block*4])
			copy(final.Pix[addr1:addr1+block*4], final.Pix[addr2:addr2+block*4])
			copy(final.Pix[addr2:addr2+block*4], exBuf[:])
			addr1 += width * 4
			addr2 += width * 4
		}
	}

	mapping := generateList(wBlocks*hBlocks, wBlocks*hBlocks, pass)

	c := 0
	for j := 0; j < hBlocks; j++ {
		for i := 0; i < wBlocks; i++ {
			_h := mapping[c] / wBlocks
			_w := mapping[c] - _h*wBlocks

			exBlock(j*block*width+i*block, _h*block*width+_w*block)

			c++
		}
	}

	f2, err := os.Create(out)
	if err != nil {
		return err
	}
	png.Encode(f2, final)

	return nil
}
