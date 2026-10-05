package splitting

import (
	"crypto/rand"
	"errors"
	"math/big"
)

var prime, _ = new(big.Int).SetString("170141183460469231731687303715884105727", 10)

type Share struct {
	X *big.Int
	Y *big.Int
}

func Split(secret string, n, k int) ([]Share, error) {
	if k > n {
		return nil, errors.New("threshold k cannot exceed total shares n")
	}

	secretInt := new(big.Int).SetBytes([]byte(secret))
	if secretInt.Cmp(prime) >= 0 {
		return nil, errors.New("secret is too large for the chosen prime field")
	}

	coefficients := make([]*big.Int, k)
	coefficients[0] = new(big.Int).Set(secretInt)

	for i := 1; i < k; i++ {
		coeff, err := rand.Int(rand.Reader, prime)
		if err != nil {
			return nil, err
		}
		coefficients[i] = coeff
	}

	shares := make([]Share, n)
	for i := 0; i < n; i++ {
		x := big.NewInt(int64(i + 1))
		y := evalPoly(coefficients, x)
		shares[i] = Share{X: new(big.Int).Set(x), Y: y}
	}

	return shares, nil
}

func Reconstruct(shares []Share) (string, error) {
	if len(shares) == 0 {
		return "", errors.New("no shares provided")
	}

	secret := big.NewInt(0)

	for i, si := range shares {
		numerator := big.NewInt(1)
		denominator := big.NewInt(1)

		for j, sj := range shares {
			if i == j {
				continue
			}

			num := new(big.Int).Neg(sj.X)
			num.Mod(num, prime)
			numerator.Mul(numerator, num)
			numerator.Mod(numerator, prime)

			den := new(big.Int).Sub(si.X, sj.X)
			den.Mod(den, prime)
			denominator.Mul(denominator, den)
			denominator.Mod(denominator, prime)
		}

		denomInv := new(big.Int).ModInverse(denominator, prime)
		if denomInv == nil {
			return "", errors.New("reconstruction failed: duplicate x values")
		}

		term := new(big.Int).Mul(si.Y, numerator)
		term.Mod(term, prime)
		term.Mul(term, denomInv)
		term.Mod(term, prime)

		secret.Add(secret, term)
		secret.Mod(secret, prime)
	}

	return string(secret.Bytes()), nil
}

func evalPoly(coefficients []*big.Int, x *big.Int) *big.Int {
	result := new(big.Int).Set(coefficients[len(coefficients)-1])

	for i := len(coefficients) - 2; i >= 0; i-- {
		result.Mul(result, x)
		result.Add(result, coefficients[i])
		result.Mod(result, prime)
	}

	return result
}
