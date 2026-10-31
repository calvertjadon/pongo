package core

import "math"

// Serve and rebound angles feed ball state, so the core evaluates them itself instead of calling math.Sin and
// math.Cos. Those functions are pure Go, but the arm64 compiler fuses their polynomial multiply-adds while the
// default amd64 build does not, so their last bits differ by architecture and an arm64 replay would diverge from an
// amd64 one. These functions evaluate the same polynomial in the same order with every product rounded explicitly,
// which both matches the amd64 results bit for bit and blocks any fusion on arm64.
//
// The inputs are bounded by maxReboundAngle, far below math's argument-reduction threshold, so the reduction below
// is the only one needed. The coefficients and the evaluation order are math's.
const (
	// piOverFourA, piOverFourB, and piOverFourC split pi/4 into three parts, from math.
	piOverFourA = 7.85398125648498535156e-1
	piOverFourB = 3.77489470793079817668e-8
	piOverFourC = 2.69515142907905952645e-15
	// fourOverPi is the octant scale math uses, computed as one untyped constant expression like math's.
	fourOverPi = 4 / math.Pi

	// sinCoefficient0 through sinCoefficient5 are math's sine polynomial coefficients, highest power first.
	sinCoefficient0 = 1.58962301576546568060e-10
	sinCoefficient1 = -2.50507477628578072866e-8
	sinCoefficient2 = 2.75573136213857245213e-6
	sinCoefficient3 = -1.98412698295895385996e-4
	sinCoefficient4 = 8.33333333332211858878e-3
	sinCoefficient5 = -1.66666666666666307295e-1

	// cosCoefficient0 through cosCoefficient5 are math's cosine polynomial coefficients, highest power first.
	cosCoefficient0 = -1.13585365213876817300e-11
	cosCoefficient1 = 2.08757008419747316778e-9
	cosCoefficient2 = -2.75573141792967388112e-7
	cosCoefficient3 = 2.48015872888517045348e-5
	cosCoefficient4 = -1.38888888888730564116e-3
	cosCoefficient5 = 4.16666666666665929218e-2
)

// sinAngle returns the sine of an angle in radians. The caller keeps the angle within the court's range.
func sinAngle(x float64) float64 {
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}
	j, z := octant(x)
	if j > 3 {
		sign = !sign
		j -= 4
	}
	value := sineSeriesOrCosineSeries(j, z)
	if sign {
		value = -value
	}

	return value
}

// cosAngle returns the cosine of an angle in radians. The caller keeps the angle within the court's range.
func cosAngle(x float64) float64 {
	if x < 0 {
		x = -x
	}
	j, z := octant(x)
	sign := false
	if j > 3 {
		j -= 4
		sign = true
	}
	if j > 1 {
		sign = !sign
	}
	value := cosineSeriesOrSineSeries(j, z)
	if sign {
		value = -value
	}

	return value
}

// octant reduces a non-negative angle within the court's range to its octant and a remainder no larger than pi/4.
func octant(x float64) (uint64, float64) {
	j := uint64(x * fourOverPi)
	whole := float64(j)
	if j&1 == 1 {
		j++
		whole++
	}
	j &= 7
	z := x - float64(whole*piOverFourA)
	z -= float64(whole * piOverFourB)
	z -= float64(whole * piOverFourC)

	return j, z
}

// sineSeriesOrCosineSeries evaluates the sine series in octants 1 and 2 and the cosine series otherwise.
func sineSeriesOrCosineSeries(j uint64, z float64) float64 {
	if j == 1 || j == 2 {
		return cosineSeries(z)
	}

	return sineSeries(z)
}

// cosineSeriesOrSineSeries evaluates the cosine series in octants 1 and 2 and the sine series otherwise.
func cosineSeriesOrSineSeries(j uint64, z float64) float64 {
	if j == 1 || j == 2 {
		return sineSeries(z)
	}

	return cosineSeries(z)
}

// sineSeries evaluates math's sine polynomial, rounding every product explicitly.
func sineSeries(z float64) float64 {
	zz := float64(z * z)
	polynomial := sinCoefficient0
	polynomial = float64(polynomial*zz) + sinCoefficient1
	polynomial = float64(polynomial*zz) + sinCoefficient2
	polynomial = float64(polynomial*zz) + sinCoefficient3
	polynomial = float64(polynomial*zz) + sinCoefficient4
	polynomial = float64(polynomial*zz) + sinCoefficient5

	return z + float64(float64(z*zz)*polynomial)
}

// cosineSeries evaluates math's cosine polynomial, rounding every product explicitly.
func cosineSeries(z float64) float64 {
	zz := float64(z * z)
	polynomial := cosCoefficient0
	polynomial = float64(polynomial*zz) + cosCoefficient1
	polynomial = float64(polynomial*zz) + cosCoefficient2
	polynomial = float64(polynomial*zz) + cosCoefficient3
	polynomial = float64(polynomial*zz) + cosCoefficient4
	polynomial = float64(polynomial*zz) + cosCoefficient5

	return float64(1-float64(0.5*zz)) + float64(float64(zz*zz)*polynomial)
}
