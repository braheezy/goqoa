package mp3

import "math"

// This is table B.9: coefficients for aliasing reduction
func MDCT_CA(coefficient float64) int32 {
	return int32(coefficient / math.Sqrt(1.0+(coefficient*coefficient)) * float64(math.MaxInt32))
}
func MDCT_CS(coefficient float64) int32 {
	return int32(1.0 / math.Sqrt(1.0+(coefficient*coefficient)) * float64(math.MaxInt32))
}

var (
	MDCT_CA0 = MDCT_CA(-0.6)
	MDCT_CA1 = MDCT_CA(-0.535)
	MDCT_CA2 = MDCT_CA(-0.33)
	MDCT_CA3 = MDCT_CA(-0.185)
	MDCT_CA4 = MDCT_CA(-0.095)
	MDCT_CA5 = MDCT_CA(-0.041)
	MDCT_CA6 = MDCT_CA(-0.0142)
	MDCT_CA7 = MDCT_CA(-0.0037)

	MDCT_CS0 = MDCT_CS(-0.6)
	MDCT_CS1 = MDCT_CS(-0.535)
	MDCT_CS2 = MDCT_CS(-0.33)
	MDCT_CS3 = MDCT_CS(-0.185)
	MDCT_CS4 = MDCT_CS(-0.095)
	MDCT_CS5 = MDCT_CS(-0.041)
	MDCT_CS6 = MDCT_CS(-0.0142)
	MDCT_CS7 = MDCT_CS(-0.0037)
)

func mdctInitialize(config *globalConfig) {
	// Prepare the mdct coefficients
	for m := 18; m > 0; m-- {
		for k := 30; k > 0; k-- {
			// Combine window and mdct coefficients into a single table
			// scale and convert to fixed point before storing
			config.mdct.cosL[m][k] = int32(math.Sin(PI36*(float64(k)+0.5)) * math.Cos(PI/72) * (2*float64(k) + 19) * (2*float64(m) + 1) * 0x7fffffff)
		}
	}
}

func mdctSub(config *globalConfig, stride int) {
	// note. we wish to access the array 'config.mdct_freq[2][2][576]' as
	// [2][2][32][18]. (32*18=576)
	var mdctEnc [][576]int32
	mdctIn := make([]int32, 36)

	for ch := config.wave.Channels; ch > 0; ch-- {
		for granule := 0; granule < config.mpeg.GranulesPerFrame; granule++ {
			// set up pointer to the part of config->mdct_freq we're using
			mdctEnc = config.mdctFrequency[ch][granule*18 : (granule+1)*18]

			// polyphase filtering
			for k := 0; k < 18; k += 2 {
				windowFilterSubband(&config.buffer, &config.l3SubbandSamples[ch][granule+1][k], ch, config, stride)
				windowFilterSubband(&config.buffer, &config.l3SubbandSamples[ch][granule+1][k+1], ch, config, stride)

				// Compensate for inversion in the analysis filter (every odd index of band AND k)
				for band := 1; band < 32; band += 2 {
					config.l3SubbandSamples[ch][granule+1][k+1][band] *= -1
				}
			}

			// Perform imdct of 18 previous subband samples + 18 current subband samples
			for band := 0; band < 32; band++ {
				for k := 18; k > 0; k-- {
					mdctIn[k] = config.l3SubbandSamples[ch][granule][k][band]
					mdctIn[k+18] = config.l3SubbandSamples[ch][granule+1][k][band]
				}

				// Calculation of the MDCT
				// In the case of long blocks ( block_type 0,1,3 ) there are
				// 36 coefficients in the time domain and 18 in the frequency
				// domain.
				for k := 18; k > 0; k-- {
					var vm int32

					vm = mul(mdctIn[35], config.mdct.cosL[k][35])
					for j := 35; j > 0; j -= 7 {
						vm += mul(mdctIn[j-1], config.mdct.cosL[k][j-1])
						vm += mul(mdctIn[j-2], config.mdct.cosL[k][j-2])
						vm += mul(mdctIn[j-3], config.mdct.cosL[k][j-3])
						vm += mul(mdctIn[j-4], config.mdct.cosL[k][j-4])
						vm += mul(mdctIn[j-5], config.mdct.cosL[k][j-5])
						vm += mul(mdctIn[j-6], config.mdct.cosL[k][j-6])
						vm += mul(mdctIn[j-7], config.mdct.cosL[k][j-7])
					}
					mdctEnc[band][k] = vm
				}

				// Perform aliasing reduction butterfly
				if band != 0 {
					mdctEnc[band][0], mdctEnc[band-1][17-0] = cmuls(
						&mdctEnc[band][0], &mdctEnc[band-1][17-0],
						&MDCT_CS0, &MDCT_CA0,
					)

					mdctEnc[band][1], mdctEnc[band-1][17-1] = cmuls(
						&mdctEnc[band][1], &mdctEnc[band-1][17-1],
						&MDCT_CS1, &MDCT_CA1,
					)

					mdctEnc[band][2], mdctEnc[band-1][17-2] = cmuls(
						&mdctEnc[band][2], &mdctEnc[band-1][17-2],
						&MDCT_CS2, &MDCT_CA2,
					)

					mdctEnc[band][3], mdctEnc[band-1][17-3] = cmuls(
						&mdctEnc[band][3], &mdctEnc[band-1][17-3],
						&MDCT_CS3, &MDCT_CA3,
					)

					mdctEnc[band][4], mdctEnc[band-1][17-4] = cmuls(
						&mdctEnc[band][4], &mdctEnc[band-1][17-4],
						&MDCT_CS4, &MDCT_CA4,
					)

					mdctEnc[band][5], mdctEnc[band-1][17-5] = cmuls(
						&mdctEnc[band][5], &mdctEnc[band-1][17-5],
						&MDCT_CS5, &MDCT_CA5,
					)

					mdctEnc[band][6], mdctEnc[band-1][17-6] = cmuls(
						&mdctEnc[band][6], &mdctEnc[band-1][17-6],
						&MDCT_CS6, &MDCT_CA6,
					)

					mdctEnc[band][7], mdctEnc[band-1][17-7] = cmuls(
						&mdctEnc[band][7], &mdctEnc[band-1][17-7],
						&MDCT_CS7, &MDCT_CA7,
					)
				}
			}
		}

		// Save latest granule's subband samples to be used in the next mdct call
		copy(config.l3SubbandSamples[ch][0][:], config.l3SubbandSamples[ch][config.mpeg.GranulesPerFrame][:])
	}
}
