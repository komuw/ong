package middleware

import (
	"testing"

	"github.com/akshayjshah/attest"
)

func TestBloom(t *testing.T) {
	t.Run("succeds", func(t *testing.T) {
		size := uint64(1000)
		hashCount := uint8(7)
		b := newBloom(size, hashCount)

		b.set("john")
		attest.True(t, b.get("john"))
		attest.False(t, b.get("joh"))
	})

	t.Run("more", func(t *testing.T) {
		size := uint64(1000)
		hashCount := uint8(7)
		b := newBloom(size, hashCount)

		animals := []string{
			"dog",
			"cat",
			"giraffe",
			"fly",
			"mosquito",
			"horse",
			"eagle",
			"bird",
			"bison",
			"boar",
			"butterfly",
			"ant",
			"anaconda",
			"bear",
			"chicken",
			"dolphin",
			"donkey",
			"crow",
			"crocodile",
		}
		for _, anim := range animals {
			b.set(anim)
		}

		for _, anim := range animals {
			attest.True(t, b.get(anim), attest.Sprintf("animal %s should have been in the bloom filter", anim))
		}

		for _, nonExistent := range []string{"Dog", "america", "germany", "cattle", "flyy", "Horse"} {
			attest.False(t, b.get(nonExistent), attest.Sprintf("item %s should NOT have been in the bloom filter", nonExistent))
		}
	})

	t.Run("goroutine safe", func(t *testing.T) {
		// maphash.Hash which we use in our bloom filter is not goroutine safe.
		// However, each bloom filter uses the same seed in its maphash.Hash so should be safe.

		size := uint64(1000)
		hashCount := uint8(7)
		b := newBloom(size, hashCount)

		item1 := "america"
		item2 := "kenya"
		item3 := "yoloo@%@$mahda8o1PkhdkJ^Q5531"
		ch := make(chan uint64, 3)
		go func() {
			ch <- b.hash(item1)
			ch <- b.hash(item2)
			ch <- b.hash(item3)
		}()

		attest.Equal(t, b.hash(item1), <-ch)
		attest.Equal(t, b.hash(item2), <-ch)
		attest.Equal(t, b.hash(item3), <-ch)
	})

	t.Run("space efficiency", func(t *testing.T) {
		words := []string{
			"dog", "cat", "giraffe", "fly", "mosquito", "horse", "eagle", "bird", "bison", "boar", "butterfly", "ant", "anaconda", "bear", "chicken", "dolphin",
			"donkey", "crow", "crocodile", "wallace", "wallet", "wallpaper", "wallpapers", "walls", "walnut", "walt", "walter", "wan", "wang", "wanna", "want",
			"wanted", "wanting", "wants", "war", "warcraft", "ward", "ware", "warehouse", "warm", "warming", "warned", "warner", "warning", "warnings", "warrant",
			"warranties", "warranty", "warren", "warrior", "warriors", "wars", "was", "wash", "washer", "washing", "washington", "waste", "watch", "watched", "watches",
			"watching", "water", "waterproof", "waters", "watershed", "watson", "watt", "watts", "wav", "wave", "waves", "wax", "way", "wayne", "ways", "wb", "wc", "we",
			"weak", "wealth", "weapon", "weapons", "wear", "wearing", "weather", "web", "webcam", "webcams", "webcast", "weblog", "weblogs", "webmaster", "webmasters",
			"webpage", "webshots", "website", "websites", "webster", "wed", "wedding", "weddings", "wednesday", "weed", "week", "weekend", "weekends", "weekly", "weeks",
			"weight", "weighted", "weights", "weird", "welcome", "welding", "welfare", "well", "wellington", "wellness", "wells", "welsh", "wendy", "went", "were",
			"wesley", "west", "western", "westminster", "wet", "whale", "what", "whatever", "whats", "wheat", "wheel", "wheels", "when", "whenever", "where",
		}
		size := uint64(len(words) / 10)
		hashCount := uint8(7)
		iterations := 200
		escape := func(v interface{}) {}

		resBloom := testing.Benchmark(func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < iterations; i++ {
				bloom := newBloom(size, hashCount)
				for _, w := range words {
					bloom.set(w)
				}
				escape(bloom)
			}
		})
		sizeBloom := resBloom.MemBytes / uint64(iterations)

		resHmap := testing.Benchmark(func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < iterations; i++ {
				hMap := map[string]struct{}{}
				for _, w := range words {
					hMap[w] = struct{}{}
				}
				escape(hMap)
			}
		})
		sizeHmap := resHmap.MemBytes / uint64(iterations)

		resSlice := testing.Benchmark(func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < iterations; i++ {
				sSlice := []string{}
				for _, w := range words {
					sSlice = append(sSlice, w)
				}
				escape(sSlice)
			}
		})
		sizeSlice := resSlice.MemBytes / uint64(iterations)

		attest.Approximately(t, sizeBloom, 309, 10)
		attest.Approximately(t, sizeHmap, 10_832, 100)
		attest.Approximately(t, sizeSlice, 8_200, 100)

		attest.True(t, sizeBloom < sizeHmap)
		attest.True(t, sizeBloom < sizeSlice)
	})
}
