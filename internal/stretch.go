// Copyright (C) 2021 Markus L. Noga
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.


package internal

import (
	"math"
)

func Stretch(f *FITSImage, autoLoc, autoScale, midtone, midBlack, gamma, ppGamma, ppSigma, scaleBlack float32) {
	// Normalize value range before applying stretches
	if f.Stats.Min==f.Stats.Max {
		LogPrintf("%d: Warning: Image is of uniform intensity %.4g, skipping normalization\n", f.ID, f.Stats.Min)
	} else {
		LogPrintf("%d: Normalizing from [%.4g,%.4g] to [0,1]\n", f.ID, f.Stats.Min, f.Stats.Max)
    	f.Normalize()
    	var err error
		f.Stats, err=CalcExtendedStats(f.Data, f.Naxisn[0])
		if err!=nil { LogFatal(err) }
	}

    // Stretch images if selected
	if autoLoc!=0 && autoScale!=0 {
		LogPrintf("%d: Auto-stretching loc to %.2f%% and scale to %.2f%% ...\n", f.ID, autoLoc, autoScale)
		StretchIterative(f, float32(autoLoc/100), float32(autoScale/100))
	}    

    // Optionally adjust midtones
    if midtone!=0 {
    	LogPrintf("Applying midtone correction with midtone=%.2f%% x scale and black=location - %.2f%% x scale\n", midtone, midBlack)

		stats,err:=CalcExtendedStats(f.Data, f.Naxisn[0])
		if err!=nil { LogFatal(err) }
		loc, scale:=stats.Location, stats.Scale
		absMid:=float32(midtone)*scale
		absBlack:=loc - float32(midBlack)*scale
    	LogPrintf("loc %.2f%% scale %.2f%% absMid %.2f%% absBlack %.2f%%\n", 100*loc, 100*scale, 100*absMid, 100*absBlack)
    	f.ApplyMidtones(absMid, absBlack)
    }

	// Optionally adjust gamma 
	if gamma!=1 {
		LogPrintf("Applying gamma %.3g\n", gamma)
		f.ApplyGamma(float32(gamma))
	}

	// Optionally adjust gamma post peak
    if ppGamma!=1 {
		stats,err:=CalcExtendedStats(f.Data, f.Naxisn[0])
		if err!=nil { LogFatal(err) }
		loc, scale:=stats.Location, stats.Scale

    	from:=loc+float32(ppSigma)*scale
    	to  :=float32(1.0)
    	LogPrintf("Based on sigma=%.4g, boosting values in [%.2f%%, %.2f%%] with gamma %.4g...\n", ppSigma, from*100, to*100, ppGamma)
		f.ApplyPartialGamma(from, to, float32(ppGamma))
    }

	// Optionally scale histogram peak
    if scaleBlack!=0 {
    	targetBlack:=float32(scaleBlack)/100.0

		stats,err:=CalcExtendedStats(f.Data, f.Naxisn[0])
		if err!=nil { LogFatal(err) }
		loc, scale:=stats.Location, stats.Scale
		LogPrintf("Location %.2f%% and scale %.2f%%: ", loc*100, scale*100)

		if loc>targetBlack {
			LogPrintf("scaling black to move location to %.2f%%...\n", targetBlack*100.0)
			f.ShiftBlackToMove(loc, targetBlack)
		} else {
			LogPrintf("cannot move to location %.2f%% by scaling black\n", targetBlack*100.0)
		}
    }
}


// Stretch image by iteratively adjusting gamma and shifting back the histogram peak
func StretchIterative(f *FITSImage, targetLoc, targetScale float32) {
	//gammaLimit:=float32(5.0)
	for i:=0; ; i++ {
		if i==50 { 
			LogPrintf("Warning: did not converge after %d iterations\n",i)
			break
		}

		// calculate basic image stats as a fast location and scale estimate
		var err error
		f.Stats,err=CalcExtendedStats(f.Data, f.Naxisn[0])
		if err!=nil { LogFatal(err) }
		loc, scale:=f.Stats.Location, f.Stats.Scale

		LogPrintf("Linear location %.2f%% and scale %.2f%%, ", loc*100, scale*100)

		if loc<=targetLoc*1.01 && scale<targetScale {
			idealGamma:=float32(1)
			idealGammaDelta:=float32(math.Abs(float64(targetScale)-float64(scale)))

			//gammaMin:=float32(1)
			//gammaMax:=float32(25)
			for gamma:=float32(1.0); gamma<=8; gamma+=0.01 {
				//gammaMid:=0.5*(gammaMin+gammaMax)
				exponent:=1.0/float64(gamma)
				newLocLower:=float32(math.Pow(float64(loc-scale), exponent))
				newLoc     :=float32(math.Pow(float64(loc        ), exponent))
				newLocUpper:=float32(math.Pow(float64(loc+scale), exponent))

				black:=(targetLoc-newLoc)/(targetLoc-1)
    			scale:=1/(1-black)

				scaledNewLocLower:=float32(math.Max(0, float64((newLocLower - black) * scale)))
				//scaledNewLoc     :=float32(math.Max(0, float64((newLoc      - black) * scale)))
				scaledNewLocUpper:=float32(math.Max(0, float64((newLocUpper - black) * scale)))

				newScale:=float32(scaledNewLocUpper-scaledNewLocLower)/2
				delta:=float32(math.Abs(float64(targetScale)-float64(newScale)))
				if delta<idealGammaDelta {
				   // LogPrintf("gamma %.2f lower %.2f%% upper %.2f%% newScale %.2f%% targetScale %.2f%% delta %.2f%% \n", gamma, scaledNewLocLower*100, scaledNewLocUpper*100, newScale*100, targetScale*100, delta*100)
					idealGamma=gamma
					idealGammaDelta=delta
				}
/*				if newScale>targetScale*1.01 {
					gammaMax=gammaMid
				} else if newScale<targetScale*1.01 {
					gammaMin=gammaMid
				} else {
					idealGamma=gammaMid
					break
				} */
			}

			//idealGamma:=float32(math.Log((float64(targetLoc)/float64(targetScale))*float64(scale))/math.Log(float64(targetLoc)))
			//if idealGamma>gammaLimit { idealGamma=gammaLimit }
			if idealGamma<=1.01 { 
				LogPrintf("done\n")
				break
			}

			LogPrintf("applying gamma %.3g\n", idealGamma)
			f.ApplyGamma(idealGamma)
		} else if loc>targetLoc*0.99 && scale<targetScale {
			LogPrintf("scaling black to move location to %.2f%%...\n", targetLoc*100)
			f.ShiftBlackToMove(loc, targetLoc)
		} else {
			LogPrintf("done\n")
			break
		}
	}
}

