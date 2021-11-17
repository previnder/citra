package citra

import "testing"

func TestImageSizeMarshal(t *testing.T) {
	list := []struct {
		size ImageSize
		want string
	}{
		{ImageSize{Width: 100, Height: 100}, "100"},
		{ImageSize{Width: 100, Height: 200}, "100x200"},
	}

	for _, item := range list {
		text, err := item.size.MarshalText()
		if err != nil || string(text) != item.want {
			t.Fatalf("ImageSize want %v, got %v (error: %v)", item.want, string(text), err)
		}
	}
}

func TestImageSizeUnmarshal(t *testing.T) {
	list := []struct {
		text                  string
		wantWidth, wantHeight int
	}{
		{"100", 100, 100},
		{"100x200", 100, 200},
	}

	size := ImageSize{}
	for _, item := range list {
		err := size.UnmarshalText([]byte(item.text))
		if err != nil || size.Width != item.wantWidth || size.Height != item.wantHeight {
			t.Fatalf("ImageSize unmarshal: want (%v, %v) got (%v, %v) (error: %v))", item.wantWidth, item.wantHeight, size.Width, size.Height, err)
		}
	}

	errors := []string{"", "100x", "xxxx", "b100x100"}

	for _, item := range errors {
		err := size.UnmarshalText([]byte(item))
		if err == nil {
			t.Fatalf("Imagesize unmarhal: want error non-nil on (%v), got nil", item)
		}
	}

}
