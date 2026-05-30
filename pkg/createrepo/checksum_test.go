package createrepo

import "testing"

func TestChecksumFile(t *testing.T) {
	tests := []struct {
		name         string
		file         string
		checksumType ChecksumType
		want         string
	}{
		{
			name:         "empty sha256",
			file:         fixturePath(t, "tests", "testdata", "test_files", "empty_file"),
			checksumType: ChecksumSHA256,
			want:         "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:         "empty sha512",
			file:         fixturePath(t, "tests", "testdata", "test_files", "empty_file"),
			checksumType: ChecksumSHA512,
			want:         "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e",
		},
		{
			name:         "text sha256",
			file:         fixturePath(t, "tests", "testdata", "test_files", "text_file"),
			checksumType: ChecksumSHA256,
			want:         "2f395bdfa2750978965e4781ddf224c89646c7d7a1569b7ebb023b170f7bd8bb",
		},
		{
			name:         "binary sha512",
			file:         fixturePath(t, "tests", "testdata", "test_files", "binary_file"),
			checksumType: ChecksumSHA512,
			want:         "339877a8ce6cdb2df62f3f76c005cac4f50144197bd095cec21056d6ddde570fe5b16e3f1cd077ece799d5dd23dc6c9c1afed018384d840bd97233c320e60dfa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ChecksumFile(tt.file, tt.checksumType)
			if err != nil {
				t.Fatalf("ChecksumFile() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ChecksumFile() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestChecksumTypeNames(t *testing.T) {
	tests := map[string]ChecksumType{
		"SHA256": ChecksumSHA256,
		"sha512": ChecksumSHA512,
		"sha":    ChecksumSHA,
		"sha1":   ChecksumSHA1,
	}
	for name, want := range tests {
		if got := ChecksumTypeFromName(name); got != want {
			t.Fatalf("ChecksumTypeFromName(%q) = %v, want %v", name, got, want)
		}
	}

	if name, ok := ChecksumSHA384.Name(); !ok || name != "sha384" {
		t.Fatalf("ChecksumSHA384.Name() = %q, %v", name, ok)
	}
	if _, ok := ChecksumUnknown.Name(); ok {
		t.Fatal("ChecksumUnknown.Name() unexpectedly succeeded")
	}
}
