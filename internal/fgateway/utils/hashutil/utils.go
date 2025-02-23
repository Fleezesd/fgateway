package hashutil

import "hash/fnv"

func HashLables(labels map[string]string) uint64 {
	finalHash := uint64(0)
	for k, v := range labels {
		// New64 returns a new 64-bit FNV-1 [hash.Hash].
		fnv := fnv.New64()
		fnv.Write([]byte(k))
		fnv.Write([]byte{0})
		fnv.Write([]byte(v))
		fnv.Write([]byte{0})
		finalHash ^= fnv.Sum64()
	}
	// make final hash
	return finalHash
}
