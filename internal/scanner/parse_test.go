package scanner

import "testing"

func TestParseMovie(t *testing.T) {
	tests := []struct {
		rel  string
		want Title
	}{
		{"Avatar (2009)/Avatar (2009).mkv", Title{"Avatar", 2009, 0}},
		{"Avatar (2009) {tmdb-19995}/Avatar (2009).mkv", Title{"Avatar", 2009, 19995}},
		{"Avatar (2009)/Avatar (2009) {tmdb-19995}.mkv", Title{"Avatar", 2009, 19995}},
		{"Batman Begins (2005).mp4", Title{"Batman Begins", 2005, 0}},
		{"Blade Runner 2049 (2017)/Blade Runner 2049 (2017) - 2160p.mkv", Title{"Blade Runner 2049", 2017, 0}},
		{"The Matrix (1999) [imdb-tt0133093]/The Matrix.mkv", Title{"The Matrix", 1999, 0}},
		{"Heat/Heat (1995).mkv", Title{"Heat", 1995, 0}},
		{"Alien.mkv", Title{"Alien", 0, 0}},
	}
	for _, tt := range tests {
		if got := ParseMovie(tt.rel); got != tt.want {
			t.Errorf("ParseMovie(%q) = %+v, want %+v", tt.rel, got, tt.want)
		}
	}
}

func TestParseEpisode(t *testing.T) {
	tests := []struct {
		rel    string
		want   Episode
		wantOK bool
	}{
		{"Grey's Anatomy (2005)/Season 01/Grey's Anatomy (2005) - s01e01 - A Hard Day's Night.mkv",
			Episode{Title{"Grey's Anatomy", 2005, 0}, 1, 1}, true},
		{"Doctor Who (2005) {tmdb-57243}/Season 10/Doctor Who (2005) - S10E12.mp4",
			Episode{Title{"Doctor Who", 2005, 57243}, 10, 12}, true},
		{"Band of Brothers/Season 1/Band.of.Brothers.S01E03.mkv",
			Episode{Title{"Band of Brothers", 0, 0}, 1, 3}, true},
		{"Show (2020)/Specials/Show (2020) - s00e01.mkv",
			Episode{Title{"Show", 2020, 0}, 0, 1}, true},
		{"Show (2020)/Season 01/Show (2020) - episode one.mkv", Episode{}, false},
		{"loose s01e01.mkv", Episode{}, false},
	}
	for _, tt := range tests {
		got, ok := ParseEpisode(tt.rel)
		if ok != tt.wantOK || (ok && got != tt.want) {
			t.Errorf("ParseEpisode(%q) = %+v, %v; want %+v, %v", tt.rel, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestIsExtra(t *testing.T) {
	tests := map[string]bool{
		"Avatar (2009)/Avatar (2009).mkv":                false,
		"Avatar (2009)/Extras/Making Of.mkv":             true,
		"Avatar (2009)/Behind The Scenes/Making Of.mkv":  true,
		"Avatar (2009)/Avatar (2009)-trailer.mp4":        true,
		"Avatar (2009)/sample.mkv":                       true,
		"Show (2020)/Season 01/Featurettes/x.mkv":        true,
		"Show (2020)/Season 01/Show (2020) - s01e01.mkv": false,
		"Other People (2016)/Other People (2016).mkv":    false,
	}
	for rel, want := range tests {
		if got := IsExtra(rel); got != want {
			t.Errorf("IsExtra(%q) = %v, want %v", rel, got, want)
		}
	}
}
