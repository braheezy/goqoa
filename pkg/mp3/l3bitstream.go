package mp3

// This is called after a frame of audio has been quantized and coded.
// It will write the encoded audio to the bitstream. Note that
// from a layer3 encoder's perspective the bit stream is primarily
// a series of main_data() blocks, with header and side information
// inserted at the proper locations to maintain framing. (See Figure A.7 in the IS).
func formatBitstream(config *globalConfig) {
	for ch := 0; ch < config.wave.Channels; ch++ {
		for gr := 0; gr < config.mpeg.GranulesPerFrame; gr++ {
			pi := &config.l3Encoding[ch][gr]
			pr := &config.mdctFrequency[ch][gr]
			for i := 0; i < GRANULE_SIZE; i++ {
				if pr[i] < 0 && pi[i] > 0 {
					pi[i] *= -1
				}
			}
		}
	}

	encodeSideInfo(config)
	encodeMainData(config)
}

func encodeMainData(config *globalConfig) {
	sideInfo := config.sideInfo
	for gr := 0; gr < config.mpeg.GranulesPerFrame; gr++ {
		for ch := 0; ch < config.wave.Channels; ch++ {
			granInfo := sideInfo.granules[gr].channels[ch]
			sLen1 := slen1Table[granInfo.ScaleFactorCompress]
			sLen2 := slen2Table[granInfo.ScaleFactorCompress]
			ix := &config.l3Encoding[ch][gr]

			if gr == 0 || sideInfo.scaleFactorSelectInfo[ch][0] == 0 {
				for sfb := 0; sfb < 6; sfb++ {
					config.bitstream.putBits(uint32(config.scaleFactor.l[gr][ch][sfb]), uint32(sLen1))
				}
			}
			if gr == 0 || sideInfo.scaleFactorSelectInfo[ch][1] == 0 {
				for sfb := 6; sfb < 11; sfb++ {
					config.bitstream.putBits(uint32(config.scaleFactor.l[gr][ch][sfb]), uint32(sLen1))
				}
			}
			if gr == 0 || sideInfo.scaleFactorSelectInfo[ch][2] == 0 {
				for sfb := 11; sfb < 16; sfb++ {
					config.bitstream.putBits(uint32(config.scaleFactor.l[gr][ch][sfb]), uint32(sLen2))
				}
			}
			if gr == 0 || sideInfo.scaleFactorSelectInfo[ch][3] == 0 {
				for sfb := 16; sfb < 21; sfb++ {
					config.bitstream.putBits(uint32(config.scaleFactor.l[gr][ch][sfb]), uint32(sLen2))
				}
			}

			huffmanCodeBits(config, ix, &granInfo)
		}
	}
}

func encodeSideInfo(config *globalConfig) {
	sideInfo := config.sideInfo

	config.bitstream.putBits(0x7ff, 11)
	config.bitstream.putBits(uint32(config.mpeg.Version), 2)
	if config.mpeg.Crc {
		config.bitstream.putBits(1, 1)
	} else {
		config.bitstream.putBits(0, 1)
	}

	config.bitstream.putBits(uint32(config.mpeg.BitrateIndex), 4)
	config.bitstream.putBits(uint32(config.mpeg.SampleRateIndex%3), 2)
	config.bitstream.putBits(uint32(config.mpeg.Padding), 1)
	config.bitstream.putBits(uint32(config.mpeg.Ext), 1)
	config.bitstream.putBits(uint32(config.mpeg.Mode), 2)
	config.bitstream.putBits(uint32(config.mpeg.ModeExt), 2)
	config.bitstream.putBits(uint32(config.mpeg.Copyright), 1)
	config.bitstream.putBits(uint32(config.mpeg.Original), 1)
	config.bitstream.putBits(uint32(config.mpeg.EmpH), 2)

	if config.mpeg.Version == MPEG_I {
		config.bitstream.putBits(0, 9)
		if config.wave.Channels == 2 {
			config.bitstream.putBits(uint32(sideInfo.privateBits), 3)
		} else {
			config.bitstream.putBits(uint32(sideInfo.privateBits), 5)
		}
	} else {
		config.bitstream.putBits(0, 8)
		if config.wave.Channels == 2 {
			config.bitstream.putBits(uint32(sideInfo.privateBits), 2)
		} else {
			config.bitstream.putBits(uint32(sideInfo.privateBits), 1)
		}
	}

	if config.mpeg.Version == MPEG_I {
		for ch := 0; ch < config.wave.Channels; ch++ {
			for scfsiBand := 0; scfsiBand < 4; scfsiBand++ {
				config.bitstream.putBits(uint32(sideInfo.scaleFactorSelectInfo[ch][scfsiBand]), 1)
			}
		}
	}

	for gr := 0; gr < config.mpeg.GranulesPerFrame; gr++ {
		for ch := 0; ch < config.wave.Channels; ch++ {
			granInfo := &sideInfo.granules[gr].channels[ch]

			config.bitstream.putBits(uint32(granInfo.Part2_3Length), 12)
			config.bitstream.putBits(uint32(granInfo.BigValues), 9)
			config.bitstream.putBits(uint32(granInfo.GlobalGain), 8)
			if config.mpeg.Version == MPEG_I {
				config.bitstream.putBits(uint32(granInfo.ScaleFactorCompress), 4)
			} else {
				config.bitstream.putBits(uint32(granInfo.ScaleFactorCompress), 9)
			}
			config.bitstream.putBits(0, 1)

			for region := 0; region < 3; region++ {
				config.bitstream.putBits(uint32(granInfo.TableSelect[region]), 5)
			}

			config.bitstream.putBits(uint32(granInfo.Region0Count), 4)
			config.bitstream.putBits(uint32(granInfo.Region1Count), 3)

			if config.mpeg.Version == MPEG_I {
				config.bitstream.putBits(uint32(granInfo.PreFlag), 1)
			}
			config.bitstream.putBits(uint32(granInfo.ScaleFactorScale), 1)
			config.bitstream.putBits(uint32(granInfo.Count1TableSelect), 1)
		}
	}
}

func huffmanCodeBits(config *globalConfig, ix *[576]int, granInfo *GranuleInfo) {
	scaleFactors := scaleFactorBandIndex[config.mpeg.SampleRateIndex]

	bits := config.bitstream.getBitsCount()

	// 1. Write the bigvalues
	bigValues := granInfo.BigValues << 1

	scaleFactorIndex := granInfo.Region0Count + 1
	region1Start := scaleFactors[scaleFactorIndex]
	scaleFactorIndex += granInfo.Region1Count + 1
	region2Start := scaleFactors[scaleFactorIndex]

	var x, y, v, w int

	for i := 0; i < int(bigValues); i += 2 {
		// get table pointer
		idx := 0
		if i >= int(region1Start) {
			idx++
		}
		if i >= int(region2Start) {
			idx++
		}

		tableIndex := granInfo.TableSelect[idx]
		// get huffman code
		if tableIndex > 0 {
			x = ix[i]
			y = ix[i+1]
			huffmanCode(&config.bitstream, tableIndex, x, y)
		}
	}

	// 2. Write count1 area
	h := huffmanCodeTable[granInfo.Count1TableSelect+32]
	count1End := bigValues + (granInfo.Count1 << 2)
	for i := bigValues; i < count1End; i += 4 {
		v = ix[i]
		w = ix[i+1]
		x = ix[i+2]
		y = ix[i+3]
		huffmanCoderCount1(&config.bitstream, &h, v, w, x, y)
	}

	bits = config.bitstream.getBitsCount() - bits
	bits = int(granInfo.Part2_3Length) - int(granInfo.Part2Length) - bits
	if bits != 0 {
		stuffingWords := bits / 32
		remainingBits := bits % 32

		// Due to the nature of the Huffman code tables, we will pad with ones
		for stuffingWords > 0 {
			config.bitstream.putBits(0xFFFFFFFF, 32)
			stuffingWords--
		}
		if remainingBits > 0 {
			config.bitstream.putBits((1<<remainingBits - 1), uint32(remainingBits))
		}
	}
}

func absAndSign(x *int) int {
	if *x > 0 {
		return 0
	}
	*x *= -1
	return 1
}

func huffmanCoderCount1(bs *Bitstream, h *huffCodeTableInfo, v, w, x, y int) {
	signV := absAndSign(&v)
	signW := absAndSign(&w)
	signX := absAndSign(&x)
	signY := absAndSign(&y)

	p := v + (w << 1) + (x << 2) + (y << 3)
	bs.putBits(uint32(h.table[p]), uint32(h.hLen[p]))

	code := uint32(0)
	cBits := 0

	if v != 0 {
		code = uint32(signV)
		cBits = 1
	}
	if w != 0 {
		code = (code << 1) | uint32(signW)
		cBits++
	}
	if x != 0 {
		code = (code << 1) | uint32(signX)
		cBits++
	}
	if y != 0 {
		code = (code << 1) | uint32(signY)
		cBits++
	}
	bs.putBits(code, uint32(cBits))
}

// Implements the pseudo-code of page 98 of the IS
func huffmanCode(bs *Bitstream, tableIndex uint, x, y int) {
	cBits, xBits := 0, 0
	code, ext := uint32(0), uint32(0)

	signX := absAndSign(&x)
	signY := absAndSign(&y)

	h := &huffmanCodeTable[tableIndex]
	yLen := h.yLen

	if tableIndex > 15 {
		// ESC-table is used
		linBitsX, linBitsY := uint32(0), uint32(0)
		linBits := int(h.linBits)

		if x > 14 {
			linBitsX = uint32(x - 15)
			x = 15
		}
		if y > 14 {
			linBitsY = uint32(y - 15)
			y = 15
		}
		idx := x*int(yLen) + y
		code = uint32(h.table[idx])
		cBits = int(h.hLen[idx])

		if x > 14 {
			ext |= linBitsX
			xBits += linBits
		}
		if x != 0 {
			ext <<= 1
			ext |= uint32(signX)
			xBits += 1
		}
		if y > 14 {
			ext <<= linBits
			ext |= linBitsY
			xBits += linBits
		}
		if y != 0 {
			ext <<= 1
			ext |= uint32(signY)
			xBits += 1
		}

		bs.putBits(code, uint32(cBits))
		bs.putBits(ext, uint32(xBits))
	} else {
		// No ESC-words
		idx := x*int(yLen) + y
		code = uint32(h.table[idx])
		cBits = int(h.hLen[idx])

		if x != 0 {
			code <<= 1
			code |= uint32(signX)
			cBits++
		}
		if y != 0 {
			code <<= 1
			code |= uint32(signY)
			cBits++
		}

		bs.putBits(code, uint32(cBits))
	}
}
