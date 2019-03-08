package brotli

/* NOLINT(build/header_guard) */
/* Copyright 2010 Google Inc. All Rights Reserved.

   Distributed under MIT license.
   See file LICENSE for detail or copy at https://opensource.org/licenses/MIT
*/
func HashTypeLengthH54() uint {
	return 8
}

func StoreLookaheadH54() uint {
	return 8
}

/* HashBytes is the function that chooses the bucket to place
   the address in. The HashLongestMatch and H54
   classes have separate, different implementations of hashing. */
func HashBytesH54(data []byte) uint32 {
	var h uint64 = ((BROTLI_UNALIGNED_LOAD64LE(data) << (64 - 8*7)) * kHashMul64)

	/* The higher bits contain more mixture from the multiplication,
	   so we take our results from there. */
	return uint32(h >> (64 - 20))
}

/* A (forgetful) hash table to the data seen by the compressor, to
   help create backward references to previous data.

   This is a hash map of fixed size ((1 << 20)). Starting from the
   given index, 4 buckets are used to store values of a key. */
type H54 struct {
	HasherCommon
	buckets_ [(1 << 20) + 4]uint32
}

func SelfH54(handle HasherHandle) *H54 {
	return handle.(*H54)
}

func InitializeH54(handle HasherHandle, params *BrotliEncoderParams) {
}

func PrepareH54(handle HasherHandle, one_shot bool, input_size uint, data []byte) {
	var self *H54 = SelfH54(handle)
	var partial_prepare_threshold uint = (4 << 20) >> 7
	/* Partial preparation is 100 times slower (per socket). */
	if one_shot && input_size <= partial_prepare_threshold {
		var i uint
		for i = 0; i < input_size; i++ {
			var key uint32 = HashBytesH54(data[i:])
			for i := 0; i < int(4); i++ {
				self.buckets_[key:][i] = 0
			}
		}
	} else {
		/* It is not strictly necessary to fill this buffer here, but
		   not filling will make the results of the compression stochastic
		   (but correct). This is because random data would cause the
		   system to find accidentally good backward references here and there. */
		self.buckets_ = [(1 << 20) + 4]uint32{}
	}
}

/* Look at 5 bytes at &data[ix & mask].
   Compute a hash from these, and store the value somewhere within
   [ix .. ix+3]. */
func StoreH54(handle HasherHandle, data []byte, mask uint, ix uint) {
	var key uint32 = HashBytesH54(data[ix&mask:])
	var off uint32 = uint32(ix>>3) % 4
	/* Wiggle the value with the bucket sweep range. */
	SelfH54(handle).buckets_[key+off] = uint32(ix)
}

func StoreRangeH54(handle HasherHandle, data []byte, mask uint, ix_start uint, ix_end uint) {
	var i uint
	for i = ix_start; i < ix_end; i++ {
		StoreH54(handle, data, mask, i)
	}
}

func StitchToPreviousBlockH54(handle HasherHandle, num_bytes uint, position uint, ringbuffer []byte, ringbuffer_mask uint) {
	if num_bytes >= HashTypeLengthH54()-1 && position >= 3 {
		/* Prepare the hashes for three last bytes of the last write.
		   These could not be calculated before, since they require knowledge
		   of both the previous and the current block. */
		StoreH54(handle, ringbuffer, ringbuffer_mask, position-3)

		StoreH54(handle, ringbuffer, ringbuffer_mask, position-2)
		StoreH54(handle, ringbuffer, ringbuffer_mask, position-1)
	}
}

func PrepareDistanceCacheH54(handle HasherHandle, distance_cache []int) {
}

/* Find a longest backward match of &data[cur_ix & ring_buffer_mask]
   up to the length of max_length and stores the position cur_ix in the
   hash table.

   Does not look for matches longer than max_length.
   Does not look for matches further away than max_backward.
   Writes the best match into |out|.
   |out|->score is updated only if a better match is found. */
func FindLongestMatchH54(handle HasherHandle, dictionary *BrotliEncoderDictionary, data []byte, ring_buffer_mask uint, distance_cache []int, cur_ix uint, max_length uint, max_backward uint, gap uint, max_distance uint, out *HasherSearchResult) {
	var self *H54 = SelfH54(handle)
	var best_len_in uint = out.len
	var cur_ix_masked uint = cur_ix & ring_buffer_mask
	var key uint32 = HashBytesH54(data[cur_ix_masked:])
	var compare_char int = int(data[cur_ix_masked+best_len_in])
	var best_score uint = out.score
	var best_len uint = best_len_in
	var cached_backward uint = uint(distance_cache[0])
	var prev_ix uint = cur_ix - cached_backward
	var bucket []uint32
	out.len_code_delta = 0
	if prev_ix < cur_ix {
		prev_ix &= uint(uint32(ring_buffer_mask))
		if compare_char == int(data[prev_ix+best_len]) {
			var len uint = FindMatchLengthWithLimit(data[prev_ix:], data[cur_ix_masked:], max_length)
			if len >= 4 {
				var score uint = BackwardReferenceScoreUsingLastDistance(uint(len))
				if best_score < score {
					best_score = score
					best_len = uint(len)
					out.len = uint(len)
					out.distance = cached_backward
					out.score = best_score
					compare_char = int(data[cur_ix_masked+best_len])
					if 4 == 1 {
						self.buckets_[key] = uint32(cur_ix)
						return
					}
				}
			}
		}
	}

	if 4 == 1 {
		var backward uint
		var len uint

		/* Only one to look for, don't bother to prepare for a loop. */
		prev_ix = uint(self.buckets_[key])

		self.buckets_[key] = uint32(cur_ix)
		backward = cur_ix - prev_ix
		prev_ix &= uint(uint32(ring_buffer_mask))
		if compare_char != int(data[prev_ix+best_len_in]) {
			return
		}

		if backward == 0 || backward > max_backward {
			return
		}

		len = FindMatchLengthWithLimit(data[prev_ix:], data[cur_ix_masked:], max_length)
		if len >= 4 {
			var score uint = BackwardReferenceScore(uint(len), backward)
			if best_score < score {
				out.len = uint(len)
				out.distance = backward
				out.score = score
				return
			}
		}
	} else {
		bucket = self.buckets_[key:]
		var i int
		prev_ix = uint(bucket[0])
		bucket = bucket[1:]
		for i = 0; i < 4; (func() { i++; tmp9 := bucket; bucket = bucket[1:]; prev_ix = uint(tmp9[0]) })() {
			var backward uint = cur_ix - prev_ix
			var len uint
			prev_ix &= uint(uint32(ring_buffer_mask))
			if compare_char != int(data[prev_ix+best_len]) {
				continue
			}

			if backward == 0 || backward > max_backward {
				continue
			}

			len = FindMatchLengthWithLimit(data[prev_ix:], data[cur_ix_masked:], max_length)
			if len >= 4 {
				var score uint = BackwardReferenceScore(uint(len), backward)
				if best_score < score {
					best_score = score
					best_len = uint(len)
					out.len = best_len
					out.distance = backward
					out.score = score
					compare_char = int(data[cur_ix_masked+best_len])
				}
			}
		}
	}

	self.buckets_[key+uint32((cur_ix>>3)%4)] = uint32(cur_ix)
}
