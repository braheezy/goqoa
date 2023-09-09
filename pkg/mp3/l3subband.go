package mp3

import "math"

// subbandInitialize calculates the analysis filterbank coefficients and rounds to the  9th decimal
// place accuracy of the filterbank tables in the ISO document. The coefficients are stored in #filter#
func subbandInitialize(config *globalConfig) {

	for i := 0; i < MAX_CHANNELS; i++ {
		config.subband.off[i] = 0
		config.subband.x[i] = [HAN_SIZE]int32{}
	}

	for i := SUBBAND_LIMIT - 1; i >= 0; i-- {
		for j := 63; j >= 0; j-- {
			filter := 1e9 * math.Cos(float64((2*i+1)*(16-j))*PI64)
			if filter >= 0 {
				filter = math.Floor(filter + 0.5)
			} else {
				filter = math.Ceil(filter - 0.5)
			}
			// Scale and convert to fixed point before storing
			config.subband.fl[i][j] = int32(filter * (0x7fffffff * 1e-9))
		}
	}
}

// Overlapping window on PCM samples 32 16-bit pcm samples are scaled to fractional 2's complement and
// concatenated to the end of the window buffer #x#. The updated window buffer #x# is then windowed by
// the analysis window #shine_enwindow# to produce the windowed sample #z# Calculates the analysis filter bank
// coefficients The windowed samples #z# is filtered by the digital filter matrix #filter# to produce the subband
// samples #s#. This done by first selectively picking out values from the windowed samples, and then
// multiplying them by the filter matrix, producing 32 subband samples.
func windowFilterSubband(buffer *[][]int16, s [SUBBAND_LIMIT]int32, ch int, config *globalConfig, stride int) {
	y := make([]int32, 64)
	ptr := (*buffer)[0]

	// Replace 32 oldest samples with 32 new samples
	for i := int32(0); i < 32; i++ {
		config.subband.x[ch][i+config.subband.off[ch]] = int32(ptr[0]) << 16
		ptr = ptr[stride:]
	}
	(*buffer)[0] = ptr

	for i := int32(64); i > 0; i-- {
		sValue := mul(config.subband.x[ch][config.subband.off[ch]+i+(0<<6)&(HAN_SIZE-1)], enWindow[i+(0<<6)])
		sValue += mul(config.subband.x[ch][config.subband.off[ch]+i+(1<<6)&(HAN_SIZE-1)], enWindow[i+(1<<6)])
		sValue += mul(config.subband.x[ch][config.subband.off[ch]+i+(2<<6)&(HAN_SIZE-1)], enWindow[i+(2<<6)])
		sValue += mul(config.subband.x[ch][config.subband.off[ch]+i+(3<<6)&(HAN_SIZE-1)], enWindow[i+(3<<6)])
		sValue += mul(config.subband.x[ch][config.subband.off[ch]+i+(4<<6)&(HAN_SIZE-1)], enWindow[i+(4<<6)])
		sValue += mul(config.subband.x[ch][config.subband.off[ch]+i+(5<<6)&(HAN_SIZE-1)], enWindow[i+(5<<6)])
		sValue += mul(config.subband.x[ch][config.subband.off[ch]+i+(6<<6)&(HAN_SIZE-1)], enWindow[i+(6<<6)])
		sValue += mul(config.subband.x[ch][config.subband.off[ch]+i+(7<<6)&(HAN_SIZE-1)], enWindow[i+(7<<6)])

		y[i] = sValue
	}

	//offset is modulo (HAN_SIZE)
	config.subband.off[ch] = (config.subband.off[ch] + 480) & (HAN_SIZE - 1)

	for i := SUBBAND_LIMIT; i > 0; i-- {
		sValue := mul(config.subband.fl[i][63], y[63])
		for j := 63; j > 0; j -= 7 {
			sValue += mul(config.subband.fl[i][j-1], y[j-1])
			sValue += mul(config.subband.fl[i][j-2], y[j-2])
			sValue += mul(config.subband.fl[i][j-3], y[j-3])
			sValue += mul(config.subband.fl[i][j-4], y[j-4])
			sValue += mul(config.subband.fl[i][j-5], y[j-5])
			sValue += mul(config.subband.fl[i][j-6], y[j-6])
			sValue += mul(config.subband.fl[i][j-7], y[j-7])
		}
		s[i] = sValue
	}
}
