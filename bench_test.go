package hh

import "testing"

func BenchmarkBaseDigest(b *testing.B) {
	address := unhex(b, testAddress)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := NewBaseDigest(address); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkKeyed(b *testing.B) {
	digest, _ := ImportBaseDigest(unhex(b, testDigestHex))
	key, err := NewSecretKey(testKeyBytes())
	if err != nil {
		b.Fatal(err)
	}
	defer key.Close()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Keyed(digest, key); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkRender(b *testing.B, fp Fingerprint, size int, opts RenderOptions) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Render(fp, size, opts); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRender128(b *testing.B) {
	universal := mustFingerprint(b, testDigestHex, ModeUniversal)
	keyed := mustFingerprint(b, testKeyedHex, ModeKeyed)
	b.Run("universal", func(b *testing.B) { benchmarkRender(b, universal, 128, RenderOptions{}) })
	b.Run("keyed", func(b *testing.B) { benchmarkRender(b, keyed, 128, RenderOptions{}) })
	b.Run("round", func(b *testing.B) {
		benchmarkRender(b, keyed, 128, RenderOptions{Shape: ShapeRound, Frame: FrameTicks, Background: Transparent()})
	})
}

func BenchmarkRender512(b *testing.B) {
	benchmarkRender(b, mustFingerprint(b, testKeyedHex, ModeKeyed), 512, RenderOptions{})
}

func BenchmarkEncodePNG128(b *testing.B) {
	img := mustRender(b, mustFingerprint(b, testKeyedHex, ModeKeyed), 128, RenderOptions{})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := img.EncodePNG(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeBMP128(b *testing.B) {
	img := mustRender(b, mustFingerprint(b, testKeyedHex, ModeKeyed), 128, RenderOptions{})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := img.EncodeBMP(White); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeJPEG128(b *testing.B) {
	img := mustRender(b, mustFingerprint(b, testKeyedHex, ModeKeyed), 128, RenderOptions{})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := img.EncodeJPEG(DefaultJPEGQuality, White); err != nil {
			b.Fatal(err)
		}
	}
}
