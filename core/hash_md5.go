package core

import (
	"crypto/md5"
	"fmt"
	"math/big"
)

// HashMD5 on js, return value in [0, 1]
// export function getHashedValue(id: string): number {
//  const FlagResult =
//    (parseInt(md5(id), 16) * 1.0) / 340282366920938463463374607431768211455
//  return FlagResult
//}

// HashMD5 represent hash function for sampling subpopulation and choose variation
func HashMD5(id string) float64 {

	hash := ""
	for _, b := range md5.Sum([]byte(id)) {
		hash += fmt.Sprintf("%02x", b)
	}

	x, _ := new(big.Int).SetString(hash, 16)
	q, _ := new(big.Int).SetString("340282366920938463463374607431768211455", 10)

	fz := new(big.Float).Quo(new(big.Float).SetInt(x), new(big.Float).SetInt(q))
	value, _ := fz.Float64()

	return value
}
