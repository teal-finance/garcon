// Copyright (C) 2020-2022 TealTicks contributors
//
// This program is free software and can be redistributed and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package main converts a S2-compressed file to a Brotli one.
package main

import (
	"flag"
	"math"
	"path/filepath"
	"time"

	"github.com/teal-finance/emo"
	"github.com/teal-finance/garcon/gg"
	"github.com/teal-finance/garcon/timex"
)

var log = emo.NewZone("reco")

const (
	minAutoLoops = 9
	maxAutoLoops = 9999
)

func main() {
	level := flag.Int("level", 99, "Compression level")
	loops := flag.Int("loops", 1, "Number of same compression times (for statistics purpose only)")
	verbose := flag.Bool("v", false, "Print weights")

	flag.Parse()
	if *loops < 1 {
		*loops = maxAutoLoops
	}

	in := flag.Arg(0)
	if in == "" {
		in = "file.s2"
	}

	ext := filepath.Ext(in)

	out := flag.Arg(1)
	if out == "" {
		dot := len(in) - len(ext)
		out = in[:dot] + gg.BrotliExt
	}

	buf := gg.Decompress(in, ext)
	log.Printf("Decompressed %v => %v", in, gg.ConvertSize(len(buf)))

	ext = filepath.Ext(out)

	durations, geometricMean := compress(*loops, buf, out, ext, *level)

	if *loops == 1 {
		return
	}

	min, arithmeticMean, variance := minAverageVariance(durations, geometricMean)

	mean := geometricMean
	for i := 0; i < 99; i++ {
		previous := mean
		mean = weightGeometricMean(durations, previous, variance, false)
		diff := math.Abs(mean - previous)
		threshold := mean / 1e4
		log.Tracef("#%d weightedGeometricMean %v diff %v threshold %v", i,
			time.Duration(mean), time.Duration(diff), time.Duration(threshold))
		if diff < threshold {
			break
		}
	}
	mean = weightGeometricMean(durations, mean, variance, *verbose)

	weightedGeometricMean := time.Duration(mean)
	log.Resultf("%d loops: Min %v WeightedGeometricMean %v GeometricMean %v ±%v ArithmeticMean %v",
		len(durations), min, weightedGeometricMean, time.Duration(geometricMean), time.Duration(variance), arithmeticMean)
}

func compress(loops int, buf []byte, fn, ext string, level int) (durations []time.Duration, geometricMean float64) {
	durations = make([]time.Duration, 0, loops)
	var sum float64
	var count int

	for i := 0; i < loops; i++ {
		d := gg.Compress(buf, fn, ext, level)
		if d <= 0 {
			log.Fatalf("Duration=%v must be > 0", d)
		}

		durations = append(durations, d)

		sum += math.Log(float64(d))
		previousMean := geometricMean
		geometricMean = math.Exp(sum / float64(i+1))

		switch {
		case loops == maxAutoLoops && i > minAutoLoops:
			diff := math.Abs(previousMean - geometricMean)
			threshold := geometricMean / 1e4
			if diff > threshold {
				log.Tracef("Compressed to %v in %v #%d geo-mean=%v diff=%v threshold=%v",
					fn, d, i+1, time.Duration(geometricMean), time.Duration(diff), time.Duration(threshold))
				count = 0
			} else {
				log.Tracef("Compressed to %v in %v #%d geo-mean=%v diff=%v threshold=%v #%v",
					fn, d, i+1, time.Duration(geometricMean), time.Duration(diff), time.Duration(threshold), count)
				count++
				if count > minAutoLoops {
					return durations, geometricMean
				}
			}

		case i > 0:
			log.Tracef("Compressed to %v in %v #%d geo-mean=%v", fn, d, i+1, time.Duration(geometricMean))

		default:
			log.Printf("Compressed to %v in %v", fn, d)
		}
	}

	return durations, geometricMean
}

func minAverageVariance(durations []time.Duration, geometricMean float64) (min, arithmeticMean time.Duration, variance float64) {
	var sum time.Duration
	var delta2Sum float64
	for _, d := range durations {
		if d < min || min == 0 {
			min = d
		}
		sum += d
		delta := (float64(d) - geometricMean)
		delta2Sum += delta * delta
	}
	arithmeticMean = sum / time.Duration(len(durations))

	// σ² = ∑(x-mean)² / n-1
	variance2 := delta2Sum / float64(len(durations)-1)
	variance = math.Sqrt(variance2)
	log.Tracef("geometricMean %v variance %v", time.Duration(geometricMean), time.Duration(variance))

	return
}

func weightGeometricMean(durations []time.Duration, mean, variance float64, doLog bool) float64 {
	var sumLogs, sumWeights float64
	min := timex.Year

	for _, d := range durations {
		var weight float64
		delta := mean - float64(d)
		delta2 := delta * delta
		if float64(d) < mean {
			weight = math.Exp(-delta2 / variance / 11) // higher value
		} else {
			weight = math.Exp(-delta2 / variance)
		}
		if doLog {
			log.Debugf("%v %v", d, weight)
		}

		sumLogs += weight * math.Log(float64(d))
		sumWeights += weight

		if min > d {
			min = d
		}
	}

	mean = math.Exp(sumLogs / sumWeights)

	if mean < float64(min) {
		log.Warningf("weightedGeometricMean < Min: %v -> %v", time.Duration(mean), min)
		return float64(min)
	}

	return mean
}
