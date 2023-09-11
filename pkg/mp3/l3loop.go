package mp3

import "math"

const (
	e                    = 2.71828182845
	cbLimit              = 21
	scaleFactorBand_LMax = 22
	enTotKrit            = 10
	enDifKrit            = 100
	enScfsiBandKrit      = 10
	xmScfsiBandKrit      = 10
)

// innerLoop selects the best quantizerStepSize for a particular set of scaleFactors.
func innerLoop(ix [GRANULE_SIZE]int, maxBits int, granuleInfo *GranuleInfo, granuleIndex int, ch int, config *globalConfig) int {
	if maxBits < 0 {
		granuleInfo.QuantizerStepSize--
	}

	var bits int

	for {
		for quantize(ix, granuleInfo.QuantizerStepSize, config) > 8192 {
			// Within table range?
			granuleInfo.QuantizerStepSize--
		}

		// rzero,count1,big_values
		calcRunLength(ix, granuleInfo)
		// count1_table selection
		bits = count1BitCount(ix, granuleInfo)
		// bigvalues sfb division
		subDivide(granuleInfo, config)
		// code book selection
		bigValuesTableSelect(ix, granuleInfo)
		// bit count
		bigValueBits := bigValuesBitCount(ix, granuleInfo)
		bits += bigValueBits

		if bits <= maxBits {
			break
		}
	}
	return bits
}

// outerLoop controls the masking conditions of all scaleFactorBands. It computes the best scaleFactor and
// global gain. This module calls the inner iteration loop.
// l3XMin: the allowed distortion of the scaleFactor.
// ix: vector of quantized values ix(0..575)
func outerLoop(maxBits int, l3XMin *PsyXMin, ix [GRANULE_SIZE]int, granuleIndex, ch int, config *globalConfig) int {
	sideInfo := &config.sideInfo
	codeInfo := &sideInfo.granules[granuleIndex].channels[ch]

	codeInfo.QuantizerStepSize = binSearchStepSize(maxBits, ix, codeInfo, config)

	codeInfo.Part2Length = calcPart2Length(granuleIndex, ch, config)
	huffmanBits := maxBits - int(codeInfo.Part2Length)

	bits := innerLoop(ix, huffmanBits, codeInfo, granuleIndex, ch, config)
	codeInfo.Part2_3Length = codeInfo.Part2Length + uint(bits)

	return int(codeInfo.Part2_3Length)
}

func iterationLoop(config *globalConfig) {

	for ch := config.wave.Channels; ch > 0; ch-- {
		for gr := 0; gr < config.mpeg.GranulesPerFrame; gr++ {
			// Setup pointers
			ix := config.l3Encoding[ch][gr]
			config.l3loop.XR = config.mdctFrequency[ch][gr][:]

			// Precalculate the square, abs, and maximum
			config.l3loop.Xrmax = 0
			for i := GRANULE_SIZE - 1; i >= 0; i-- {
				config.l3loop.Xrsq[i] = mulSR(config.l3loop.XR[i], config.l3loop.XR[i])
				config.l3loop.Xrabs[i] = int32(math.Abs(float64(config.l3loop.XR[i])))
				if config.l3loop.Xrabs[i] > config.l3loop.Xrmax {
					config.l3loop.Xrmax = config.l3loop.Xrabs[i]
				}
			}

			codeInfo := &config.sideInfo.granules[gr].channels[ch]
			codeInfo.ScaleFactorBandMaxLen = scaleFactorBand_LMax - 1

			l3XMin := PsyXMin{}
			calcXMin(&config.ratio, codeInfo, &l3XMin, gr, ch)

			if config.mpeg.Version == MPEG_I {
				calcSCFSI(&l3XMin, ch, gr, config)
			}

			// Calculation of the number of available bits per granule
			maxBits := maxReservoirBits(config.psychoEnergy[ch][gr], config)

			// Reset iteration variables
			for gr := 0; gr < MAX_GRANULES; gr++ {
				for ch := 0; ch < MAX_CHANNELS; ch++ {
					for i := 0; i < 22; i++ {
						config.scaleFactor.l[gr][ch][i] = 0
					}
					for j := 0; j < 13; j++ {
						for k := 0; k < 3; k++ {
							config.scaleFactor.s[gr][ch][j][k] = 0
						}
					}
				}
			}

			codeInfo.Part2_3Length = 0
			codeInfo.BigValues = 0
			codeInfo.Count1 = 0
			codeInfo.ScaleFactorCompress = 0
			codeInfo.TableSelect[0] = 0
			codeInfo.TableSelect[1] = 0
			codeInfo.TableSelect[2] = 0
			codeInfo.Region0Count = 0
			codeInfo.Region1Count = 0
			codeInfo.Part2Length = 0
			codeInfo.PreFlag = 0
			codeInfo.ScaleFactorScale = 0
			codeInfo.Count1TableSelect = 0

			// All spectral values zero?
			if config.l3loop.Xrmax > 0 {
				codeInfo.Part2_3Length = uint(outerLoop(maxBits, &l3XMin, ix, gr, ch, config))
			}

			reservoirAdjust(codeInfo, config)
			codeInfo.GlobalGain = uint(codeInfo.QuantizerStepSize + 210)
		}
	}

	reservoirFrameEnd(config)
}

// loopInitialize calculates the look up tables used by the iteration loop.
func loopInitialize(config *globalConfig) {
	// quantize: stepsize conversion, fourth root of 2 table.
	// The table is inverted (negative power) from the equation given
	// in the spec because it is quicker to do x*y than x/y.
	// The 0.5 is for rounding.
	for i := 128; i > 0; i-- {
		config.l3loop.StepTable[i] = math.Pow(2.0, float64(127-i)/4)
		if config.l3loop.StepTable[i]*2 > 0x7fffffff {
			config.l3loop.StepTableI[i] = 0x7fffffff
		} else {
			// The table is multiplied by 2 to give an extra bit of accuracy.
			// In quantize, the long multiply does not shift it's result left one
			// bit to compensate.
			config.l3loop.StepTableI[i] = int32((config.l3loop.StepTable[i] * 2) + 0.5)
		}
	}

	// quantize: vector conversion, three quarter power table.
	// The 0.5 is for rounding, the .0946 comes from the spec.
	for i := 10000; i > 0; i-- {
		config.l3loop.Int2idx[i] = int(math.Sqrt(math.Sqrt(float64(i)*float64(i)) - 0.0946 + 0.5))
	}
}

// calcSCFSI calculates the scalefactor select information ( scfsi )
func calcSCFSI(l3XMin *PsyXMin, ch, granuleIndex int, config *globalConfig) {
	l3Side := &config.sideInfo
	// This is the scfsi_band table from 2.4.2.7 of the IS
	scfsiBandLong := [5]int{0, 6, 11, 16, 21}

	condition := 0
	scaleFactorBandLong := scaleFactorBandIndex[config.mpeg.SampleRateIndex]

	config.l3loop.Xrmaxl[granuleIndex] = config.l3loop.Xrmax

	temp := 0
	// the total energy of the granule
	for i := GRANULE_SIZE; i > 0; i-- {
		// a bit of scaling to avoid overflow, (not very good)
		temp += int(config.l3loop.Xrsq[i] >> 10)
	}
	if temp != 0 {
		config.l3loop.EnTot[granuleIndex] = int32(math.Log(float64(temp)*4.768371584e-7) / LN2)
	} else {
		config.l3loop.EnTot[granuleIndex] = 0
	}

	// the energy of each scalefactor band, en
	// the allowed distortion of each scalefactor band, xm
	for sfb := 21; sfb > 0; sfb-- {
		start := scaleFactorBandLong[sfb]
		end := scaleFactorBandLong[sfb+1]

		var i int32
		for temp, i = 0, start; i < end; i++ {
			temp += int(config.l3loop.Xrsq[i] >> 10)
		}
		if temp != 0 {
			config.l3loop.En[granuleIndex][sfb] = int32(math.Log(float64(temp)*4.768371584e-7) / LN2)
		} else {
			config.l3loop.En[granuleIndex][sfb] = 0
		}
	}

	if granuleIndex == 1 {
		for gr := 2; gr > 0; gr-- {
			// the spectral values are not all zero
			if config.l3loop.Xrmaxl[gr] != 0 {
				condition++
			}
			condition++
		}
		if math.Abs(float64(config.l3loop.EnTot[0])-float64(config.l3loop.EnTot[1])) < enTotKrit {
			condition++
		}
		tp := 0.0
		for sfb := 21; sfb > 0; sfb-- {
			tp += math.Abs(float64(config.l3loop.En[0][sfb] - config.l3loop.En[1][sfb]))
		}
		if tp < enDifKrit {
			condition++
		}

		if condition == 6 {
			for scfsiBand := 0; scfsiBand < 4; scfsiBand++ {
				sum0, sum1 := 0, 0
				l3Side.scaleFactorSelectInfo[ch][scfsiBand] = 0
				start := scfsiBandLong[scfsiBand]
				end := scfsiBandLong[scfsiBand+1]
				for sfb := start; sfb < end; sfb++ {
					sum0 += int(math.Abs(float64(config.l3loop.En[0][sfb] - config.l3loop.En[1][sfb])))
					sum1 += int(math.Abs(float64(config.l3loop.Xm[0][sfb] - config.l3loop.Xm[1][sfb])))
				}

				if sum0 < enScfsiBandKrit && sum1 < xmScfsiBandKrit {
					l3Side.scaleFactorSelectInfo[ch][scfsiBand] = 1
				} else {
					l3Side.scaleFactorSelectInfo[ch][scfsiBand] = 0
				}
			}
		} else {
			for scfsiBand := 0; scfsiBand < 4; scfsiBand++ {
				l3Side.scaleFactorSelectInfo[ch][scfsiBand] = 0
			}
		}
	}
}

// calcPart2Length calculates the number of bits needed to encode the scaleFactor in the
// main data block.
func calcPart2Length(granuleIndex, ch int, config *globalConfig) uint {
	granuleInfo := &config.sideInfo.granules[granuleIndex].channels[ch]

	bits := uint(0)

	sLen1 := slen1Table[granuleInfo.ScaleFactorCompress]
	sLen2 := slen2Table[granuleInfo.ScaleFactorCompress]

	if granuleIndex == 0 || config.sideInfo.scaleFactorSelectInfo[ch][0] == 0 {
		bits += uint(sLen1 * 6)
	}
	if granuleIndex == 0 || config.sideInfo.scaleFactorSelectInfo[ch][1] == 0 {
		bits += uint(sLen1 * 5)
	}
	if granuleIndex == 0 || config.sideInfo.scaleFactorSelectInfo[ch][2] == 0 {
		bits += uint(sLen2 * 5)
	}
	if granuleIndex == 0 || config.sideInfo.scaleFactorSelectInfo[ch][3] == 0 {
		bits += uint(sLen2 * 5)
	}
	return bits
}

// calcXMin calculates the allowed distortion for each scalefactor band,
// as determined by the psychoacoustic model. xmin(sb) = ratio(sb) * en(sb) / bw(sb)
func calcXMin(ratio *PsyRatio, codeInfo *GranuleInfo, l3XMin *PsyXMin, granuleIndex, ch int) {
	for sfb := codeInfo.ScaleFactorBandMaxLen; sfb > 0; sfb-- {
		// NB: xmin will always be zero with no psychoacoustic model...
		l3XMin.l[granuleIndex][ch][sfb] = 0
	}
}

// binSearchStepSize successively approximates an approach to obtaining a initial quantizer
// step size. The following optional code written by Seymour Shlien will speed up the shine_outer_loop code which is
// called by iteration_loop. When BIN_SEARCH is defined, the shine_outer_loop function precedes the call to the
// function shine_inner_loop with a call to bin_search gain defined below, which returns a good starting quantizerStepSize.
func binSearchStepSize(desiredRate int, ix [GRANULE_SIZE]int, codeInfo *GranuleInfo, config *globalConfig) int {
	next := -120
	count := 120

	var bit int

	for {
		half := count / 2

		if quantize(ix, next+half, config) > 8192 {
			// fail
			bit = 100000
		} else {
			calcRunLength(ix, codeInfo)
			bit = count1BitCount(ix, codeInfo)
			subDivide(codeInfo, config)
			bigValuesTableSelect(ix, codeInfo)
			bit += bigValuesBitCount(ix, codeInfo)
		}

		if bit < desiredRate {
			count = half
		} else {
			next += half
			count -= half
		}

		if count <= 1 {
			break
		}
	}
	return next
}

// countBit counts the number of bits necessary to code the subregion.
func countBit(ix [GRANULE_SIZE]int, start, end, table uint32) int {
	if table == 0 {
		return 0
	}

	h := &huffmanCodeTable[table]
	sum := 0

	yLen := h.yLen
	linBits := h.linBits

	if table > 15 {
		//ESC-table is used
		for i := start; i < end; i += 2 {
			x := ix[i]
			y := ix[i+1]
			if x > 14 {
				x = 15
				sum += int(linBits)
			}
			if y > 14 {
				y = 15
				sum += int(linBits)
			}

			sum += int(h.hLen[x*int(yLen)+y])
			if x != 0 {
				sum++
			}
			if y != 0 {
				sum++
			}
		}
	} else {
		// No ESC words
		for i := start; i < end; i += 2 {
			x := ix[i]
			y := ix[i+1]

			sum += int(h.hLen[x*int(yLen)+y])
			if x != 0 {
				sum++
			}
			if y != 0 {
				sum++
			}
		}
	}
	return sum
}

// bigValuesBitCount Count the number of bits necessary to code the bigValues region.
func bigValuesBitCount(ix [GRANULE_SIZE]int, granuleInfo *GranuleInfo) int {
	bits := 0

	if table := granuleInfo.TableSelect[0]; table != 0 {
		// region0
		bits += countBit(ix, 0, uint32(granuleInfo.Address1), uint32(table))
	}
	if table := granuleInfo.TableSelect[1]; table != 0 {
		// region1
		bits += countBit(ix, uint32(granuleInfo.Address1), uint32(granuleInfo.Address1), uint32(table))
	}
	if table := granuleInfo.TableSelect[0]; table != 0 {
		// region2
		bits += countBit(ix, uint32(granuleInfo.Address2), uint32(granuleInfo.Address3), uint32(table))
	}

	return bits
}

// quantize perform quantization of the vector xr ( -> ix).
// Returns maximum value of ix
func quantize(ix [GRANULE_SIZE]int, stepSize int, config *globalConfig) int {
	//  2**(-stepSize/4)
	scaleI := config.l3loop.StepTableI[stepSize+127]

	max := 0

	// a quick check to see if ixMax will be less than 8192
	// this speeds up the early calls to binSearchStepSize
	// 8192**(4/3) == 165140
	if mulR(config.l3loop.Xrmax, scaleI) > 165140 {
		// no point in continuing, stepSize not big enough
		max = 16384
	} else {
		for i := 0; i < GRANULE_SIZE; i++ {
			// This calculation is very sensitive. The multiply must round it's
			// result or bad things happen to the quality.
			ln := mulR(int32(math.Abs(float64(config.l3loop.XR[i]))), scaleI)

			// ln < 10000 catches most values
			if ln < 10000 {
				// Use quick look up method
				ix[i] = config.l3loop.Int2idx[ln]
			} else {
				// outside table range so have to do it using floats
				//  2**(-stepSize/4)
				scale := config.l3loop.StepTable[stepSize+127]
				dbl := float64(config.l3loop.Xrabs[i]) * scale * 4.656612875e-10
				// dbl**(3/4)
				ix[i] = int(math.Sqrt(math.Sqrt(dbl) * dbl))
			}

			// calculate ixMax while we're here
			// note. ix cannot be negative */
			if ix[i] > max {
				max = ix[i]
			}
		}
	}

	return max
}

// bigValuesTableSelect selects huffman code tables for the bigValues region
func bigValuesTableSelect(ix [GRANULE_SIZE]int, codeInfo *GranuleInfo) {
	codeInfo.TableSelect[0] = 0
	codeInfo.TableSelect[1] = 0
	codeInfo.TableSelect[2] = 0

	if codeInfo.Address1 > 0 {
		codeInfo.TableSelect[0] = newChooseTable(ix, 0, uint32(codeInfo.Address1))
	}
	if codeInfo.Address2 > codeInfo.Address1 {
		codeInfo.TableSelect[1] = newChooseTable(ix, uint32(codeInfo.Address1), uint32(codeInfo.Address2))
	}
	if codeInfo.BigValues<<1 > codeInfo.Address2 {
		codeInfo.TableSelect[2] = newChooseTable(ix, uint32(codeInfo.Address2), uint32(codeInfo.BigValues<<1))
	}
}

// count1BitCount determines the number of bits to encode the quadruples.
func count1BitCount(ix [GRANULE_SIZE]int, codeInfo *GranuleInfo) int {
	sum0 := 0
	sum1 := 0
	for i, k := codeInfo.BigValues<<1, uint(0); k < codeInfo.Count1; i += 4 {
		k++

		v := ix[i]
		w := ix[i+1]
		x := ix[i+2]
		y := ix[i+3]

		p := v + (w << 1) + (x << 2) + (y << 3)

		signBits := 0
		if v != 0 {
			signBits++
		}
		if w != 0 {
			signBits++
		}
		if x != 0 {
			signBits++
		}
		if y != 0 {
			signBits++
		}

		sum0 += signBits
		sum1 += signBits

		sum0 += int(huffmanCodeTable[32].hLen[p])
		sum1 += int(huffmanCodeTable[33].hLen[p])
	}

	if sum0 < sum1 {
		codeInfo.Count1TableSelect = 0
		return sum0
	} else {
		codeInfo.Count1TableSelect = 1
		return sum1
	}
}

// calcRunLength calculates rZero, count1, big_values (Partitions ix into big values, quadruples and zeros)
func calcRunLength(ix [GRANULE_SIZE]int, codeInfo *GranuleInfo) {
	rZero := 0

	i := GRANULE_SIZE

	for ; i > 1; i -= 2 {
		if ix[i-1] == 0 && ix[i-2] == 0 {
			rZero++
		} else {
			break
		}
	}

	codeInfo.Count1 = 0
	for ; i > 3; i -= 4 {
		if ix[i-1] <= 1 && ix[i-2] <= 1 && ix[i-3] <= 1 && ix[i-4] <= 1 {
			codeInfo.Count1++
		} else {
			break
		}
	}

	codeInfo.BigValues = uint(i >> 1)
}

// ixMax calculates the maximum of ix from 0 to 575
func ixMax(ix [GRANULE_SIZE]int, begin, end uint32) (max int) {
	for i := begin; i < end; i++ {
		if ix[i] > max {
			max = ix[i]
		}
	}
	return max
}

// newChooseTable chooses the Huffman table that will encode ix[begin..end] with the fewest bits.
// Note: This code contains knowledge about the sizes and characteristics of the Huffman tables as
// defined in the IS (Table B.7), and will not work with any arbitrary tables.
func newChooseTable(ix [GRANULE_SIZE]int, begin, end uint32) uint {
	max := uint32(ixMax(ix, begin, end))
	if max == 0 {
		return 0
	}

	choice := [2]int{}
	sum := [2]int{}

	if max < 15 {
		// Try tables with no linbits
		for i := 14; i >= 0; i-- {
			if huffmanCodeTable[i].xLen > max {
				choice[0] = i
				break
			}
		}

		sum[0] = countBit(ix, begin, end, uint32(choice[0]))

		switch choice[0] {
		case 2:
			sum[1] = countBit(ix, begin, end, 3)
			if sum[1] <= sum[0] {
				choice[0] = 3
			}
		case 5:
			sum[1] = countBit(ix, begin, end, 6)
			if sum[1] <= sum[0] {
				choice[0] = 6
			}
		case 7:
			sum[1] = countBit(ix, begin, end, 8)
			if sum[1] <= sum[0] {
				choice[0] = 8
				sum[0] = sum[1]
			}
			sum[1] = countBit(ix, begin, end, 9)
			if sum[1] <= sum[0] {
				choice[0] = 9
			}
		case 10:
			sum[1] = countBit(ix, begin, end, 11)
			if sum[1] <= sum[0] {
				choice[0] = 11
				sum[0] = sum[1]
			}
			sum[1] = countBit(ix, begin, end, 12)
			if sum[1] <= sum[0] {
				choice[0] = 12
			}
		case 13:
			sum[1] = countBit(ix, begin, end, 15)
			if sum[1] <= sum[0] {
				choice[0] = 15
			}
		}
	} else {
		// Try tables with linbits
		max -= 15

		for i := 15; i < 24; i++ {
			if huffmanCodeTable[i].linMax >= max {
				choice[0] = i
				break
			}
		}

		for i := 24; i < 32; i++ {
			if huffmanCodeTable[i].linMax >= max {
				choice[1] = i
				break
			}
		}

		sum[0] = countBit(ix, begin, end, uint32(choice[0]))
		sum[1] = countBit(ix, begin, end, uint32(choice[1]))
		if sum[1] < sum[0] {
			choice[0] = choice[1]
		}
	}
	return uint(choice[0])
}

// subDivide subdivides the bigValue region which will use separate Huffman tables.
func subDivide(codeInfo *GranuleInfo, config *globalConfig) {
	type subRegion struct {
		region0Count uint32
		region1Count uint32
	}

	var subDvTable = [23]subRegion{
		{0, 0}, /* 0 bands */
		{0, 0}, /* 1 bands */
		{0, 0}, /* 2 bands */
		{0, 0}, /* 3 bands */
		{0, 0}, /* 4 bands */
		{0, 1}, /* 5 bands */
		{1, 1}, /* 6 bands */
		{1, 1}, /* 7 bands */
		{1, 2}, /* 8 bands */
		{2, 2}, /* 9 bands */
		{2, 3}, /* 10 bands */
		{2, 3}, /* 11 bands */
		{3, 4}, /* 12 bands */
		{3, 4}, /* 13 bands */
		{3, 4}, /* 14 bands */
		{4, 5}, /* 15 bands */
		{4, 5}, /* 16 bands */
		{4, 6}, /* 17 bands */
		{5, 6}, /* 18 bands */
		{5, 6}, /* 19 bands */
		{5, 7}, /* 20 bands */
		{6, 7}, /* 21 bands */
		{6, 7}, /* 22 bands */
	}

	if codeInfo.BigValues == 0 {
		// no bigValues region
		codeInfo.Region0Count = 0
		codeInfo.Region1Count = 0
	} else {
		scaleFactorBandLong := scaleFactorBandIndex[config.mpeg.SampleRateIndex][:]
		bigValuesRegion := codeInfo.BigValues * 2

		scaleFactorBandCountAnalysis := 0
		for scaleFactorBandLong[scaleFactorBandCountAnalysis] < int32(bigValuesRegion) {
			scaleFactorBandCountAnalysis++
		}

		var thisCount uint32
		for thisCount = subDvTable[scaleFactorBandCountAnalysis].region0Count; thisCount > 0; thisCount-- {
			if scaleFactorBandLong[thisCount+1] <= int32(bigValuesRegion) {
				break
			}
		}
		codeInfo.Region0Count = uint(thisCount)
		codeInfo.Address1 = uint(scaleFactorBandLong[thisCount+1])

		scaleFactorBandLong = scaleFactorBandLong[codeInfo.Region0Count+1:]

		for thisCount = subDvTable[scaleFactorBandCountAnalysis].region1Count; thisCount > 0; thisCount-- {
			if scaleFactorBandLong[thisCount+1] <= int32(bigValuesRegion) {
				break
			}
		}
		codeInfo.Region1Count = uint(thisCount)
		codeInfo.Address2 = uint(scaleFactorBandLong[thisCount+1])

		codeInfo.Address3 = uint(bigValuesRegion)
	}
}
