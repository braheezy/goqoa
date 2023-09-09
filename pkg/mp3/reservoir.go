// Layer3 bit reservoir: Described in C.1.5.4.2.2 of the IS
package mp3

// maxReservoirBits is called at the beginning of each granule to get the max bit
// allowance for the current granule based on reservoir size and perceptual entropy.
func maxReservoirBits(pe float64, config *globalConfig) int {
	meanBits := config.meanBits

	meanBits /= config.wave.Channels
	maxBits := meanBits

	if maxBits > 4095 {
		maxBits = 4095
	}
	if config.reservoirMaxSize == 0 {
		return maxBits
	}
	moreBits := int(pe*3.1) - meanBits
	addBits := 0

	if moreBits > 100 {
		frac := config.reservoirSize * 6.0 / 10.0
		if frac < moreBits {
			addBits = frac
		} else {
			addBits = moreBits
		}
	}
	overBits := config.reservoirSize - (config.reservoirMaxSize<<3)/10 - int(addBits)
	if overBits > 0 {
		addBits += overBits
	}

	maxBits += int(addBits)
	if maxBits > 4095 {
		maxBits = 4095
	}
	return maxBits

}

// reservoirAdjust is called after a granule's bit allocation. It readjusts the size of
// the reservoir to reflect the granule's usage.
func reservoirAdjust(granuleInfo *GranuleInfo, config *globalConfig) {
	config.reservoirSize += config.meanBits/config.wave.Channels - int(granuleInfo.Part2_3Length)
}

func reservoirFrameEnd(config *globalConfig) {
	ancillaryPad := 0

	// Just in case meanBits is odd, this is necessary
	if config.wave.Channels == 2 && config.meanBits&1 != 0 {
		config.reservoirSize++
	}
	overBits := config.reservoirSize - config.reservoirMaxSize
	if overBits < 0 {
		overBits = 0
	}

	config.reservoirSize -= overBits
	stuffingBits := overBits + ancillaryPad

	// We must be byte-aligned
	if overBits = config.reservoirSize % 8; overBits != 0 {
		stuffingBits += overBits
		config.reservoirSize -= overBits
	}

	if stuffingBits > 0 {
		l3Side := &config.sideInfo
		// Plan A: put all stuffingBits into the first granule
		granInfo := &l3Side.granules[0].channels[0]
		if granInfo.Part2_3Length+uint(stuffingBits) < 4095 {
			granInfo.Part2_3Length += uint(stuffingBits)
		} else {
			// Plan B: distribute the stuffingBits evenly over the granules
			for gr := 0; gr < config.mpeg.GranulesPerFrame; gr++ {
				for ch := 0; ch < config.wave.Channels; ch++ {
					if stuffingBits == 0 {
						break
					}

					granInfo = &l3Side.granules[gr].channels[ch]
					extraBits := 4095 - granInfo.Part2_3Length
					bitsThisGranule := extraBits
					if bitsThisGranule >= uint(stuffingBits) {
						bitsThisGranule = uint(stuffingBits)
					}
					granInfo.Part2_3Length += bitsThisGranule
					stuffingBits -= int(bitsThisGranule)
				}
			}

			// If any stuffing bits remain, we elect to spill them into into ancillary data.
			// The bitstream formatter will do this if l3side.reservoirDrain is set.
			l3Side.reservoirDrain = stuffingBits
		}
	}
}
