package tmdb

import "encoding/json"

// ---------------------------------------------------------------- shared

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Company struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

type Country struct {
	ISO31661 string `json:"iso_3166_1"`
	Name     string `json:"name"`
}

type SpokenLanguage struct {
	ISO6391     string `json:"iso_639_1"`
	EnglishName string `json:"english_name"`
	Name        string `json:"name"`
}

type Keyword struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Keywords: movies use "keywords", tv uses "results".
type Keywords struct {
	Keywords []Keyword `json:"keywords"`
	Results  []Keyword `json:"results"`
}

func (k *Keywords) All() []Keyword {
	if k == nil {
		return nil
	}
	return append(append([]Keyword{}, k.Keywords...), k.Results...)
}

type Person struct {
	ID                 int     `json:"id"`
	Name               string  `json:"name"`
	OriginalName       string  `json:"original_name"`
	Gender             int     `json:"gender"`
	KnownForDepartment string  `json:"known_for_department"`
	ProfilePath        string  `json:"profile_path"`
	Popularity         float64 `json:"popularity"`
	Adult              bool    `json:"adult"`
}

type CastMember struct {
	Person
	CastID    int    `json:"cast_id"`
	Character string `json:"character"`
	CreditID  string `json:"credit_id"`
	Order     int    `json:"order"`
}

type CrewMember struct {
	Person
	CreditID   string `json:"credit_id"`
	Department string `json:"department"`
	Job        string `json:"job"`
}

type Credits struct {
	Cast       []CastMember `json:"cast"`
	Crew       []CrewMember `json:"crew"`
	GuestStars []CastMember `json:"guest_stars"`
}

type AggregateRole struct {
	CreditID     string `json:"credit_id"`
	Character    string `json:"character"`
	EpisodeCount int    `json:"episode_count"`
}

type AggregateJob struct {
	CreditID     string `json:"credit_id"`
	Job          string `json:"job"`
	EpisodeCount int    `json:"episode_count"`
}

type AggregateCast struct {
	Person
	Roles             []AggregateRole `json:"roles"`
	TotalEpisodeCount int             `json:"total_episode_count"`
	Order             int             `json:"order"`
}

type AggregateCrew struct {
	Person
	Jobs              []AggregateJob `json:"jobs"`
	Department        string         `json:"department"`
	TotalEpisodeCount int            `json:"total_episode_count"`
}

type AggregateCredits struct {
	Cast []AggregateCast `json:"cast"`
	Crew []AggregateCrew `json:"crew"`
}

type Image struct {
	FilePath    string  `json:"file_path"`
	AspectRatio float64 `json:"aspect_ratio"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	ISO6391     string  `json:"iso_639_1"`
	ISO31661    string  `json:"iso_3166_1"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
}

type Images struct {
	Backdrops []Image `json:"backdrops"`
	Logos     []Image `json:"logos"`
	Posters   []Image `json:"posters"`
	Stills    []Image `json:"stills"`
}

type Video struct {
	ID          string `json:"id"`
	ISO6391     string `json:"iso_639_1"`
	ISO31661    string `json:"iso_3166_1"`
	Name        string `json:"name"`
	Key         string `json:"key"`
	Site        string `json:"site"`
	Size        int    `json:"size"`
	Type        string `json:"type"`
	Official    bool   `json:"official"`
	PublishedAt string `json:"published_at"`
}

type Videos struct {
	Results []Video `json:"results"`
}

type AltTitle struct {
	ISO31661 string `json:"iso_3166_1"`
	Title    string `json:"title"`
	Type     string `json:"type"`
}

// AltTitles: movies use "titles", tv uses "results".
type AltTitles struct {
	Titles  []AltTitle `json:"titles"`
	Results []AltTitle `json:"results"`
}

func (a *AltTitles) All() []AltTitle {
	if a == nil {
		return nil
	}
	return append(append([]AltTitle{}, a.Titles...), a.Results...)
}

type Translation struct {
	ISO31661    string `json:"iso_3166_1"`
	ISO6391     string `json:"iso_639_1"`
	Name        string `json:"name"`
	EnglishName string `json:"english_name"`
	Data        struct {
		Title    string `json:"title"`
		Name     string `json:"name"`
		Overview string `json:"overview"`
		Homepage string `json:"homepage"`
		Tagline  string `json:"tagline"`
		Runtime  int    `json:"runtime"`
	} `json:"data"`
}

type Translations struct {
	Translations []Translation `json:"translations"`
}

// Summary is a movie or tv list item (search results, recommendations,
// similar, collection parts).
type Summary struct {
	ID            int     `json:"id"`
	MediaType     string  `json:"media_type"`
	Title         string  `json:"title"`
	Name          string  `json:"name"`
	OriginalTitle string  `json:"original_title"`
	OriginalName  string  `json:"original_name"`
	Overview      string  `json:"overview"`
	ReleaseDate   string  `json:"release_date"`
	FirstAirDate  string  `json:"first_air_date"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	Popularity    float64 `json:"popularity"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
}

// Paged keeps every result as raw JSON so it can be stored verbatim.
type Paged struct {
	Page         int               `json:"page"`
	TotalPages   int               `json:"total_pages"`
	TotalResults int               `json:"total_results"`
	Results      []json.RawMessage `json:"results"`
}

type Provider struct {
	ProviderID      int    `json:"provider_id"`
	ProviderName    string `json:"provider_name"`
	LogoPath        string `json:"logo_path"`
	DisplayPriority int    `json:"display_priority"`
}

type RegionProviders struct {
	Link     string     `json:"link"`
	Flatrate []Provider `json:"flatrate"`
	Rent     []Provider `json:"rent"`
	Buy      []Provider `json:"buy"`
	Ads      []Provider `json:"ads"`
	Free     []Provider `json:"free"`
}

type WatchProviders struct {
	Results map[string]RegionProviders `json:"results"`
}

type Review struct {
	ID            string `json:"id"`
	Author        string `json:"author"`
	AuthorDetails struct {
		Name       string   `json:"name"`
		Username   string   `json:"username"`
		AvatarPath string   `json:"avatar_path"`
		Rating     *float64 `json:"rating"`
	} `json:"author_details"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	URL       string `json:"url"`
}

type Reviews struct {
	Results []Review `json:"results"`
}

// ---------------------------------------------------------------- movies

type List struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	FavoriteCount int    `json:"favorite_count"`
	ItemCount     int    `json:"item_count"`
	ISO6391       string `json:"iso_639_1"`
	ListType      string `json:"list_type"`
	PosterPath    string `json:"poster_path"`
}

type Lists struct {
	Results []List `json:"results"`
}

type ReleaseDates struct {
	Results []struct {
		ISO31661     string `json:"iso_3166_1"`
		ReleaseDates []struct {
			Certification string   `json:"certification"`
			Descriptors   []string `json:"descriptors"`
			ISO6391       string   `json:"iso_639_1"`
			Note          string   `json:"note"`
			ReleaseDate   string   `json:"release_date"`
			Type          int      `json:"type"`
		} `json:"release_dates"`
	} `json:"results"`
}

type CollectionRef struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
}

type Movie struct {
	ID                  int              `json:"id"`
	IMDbID              string           `json:"imdb_id"`
	Title               string           `json:"title"`
	OriginalTitle       string           `json:"original_title"`
	OriginalLanguage    string           `json:"original_language"`
	Overview            string           `json:"overview"`
	Tagline             string           `json:"tagline"`
	Status              string           `json:"status"`
	ReleaseDate         string           `json:"release_date"`
	Runtime             int              `json:"runtime"`
	Budget              int64            `json:"budget"`
	Revenue             int64            `json:"revenue"`
	Popularity          float64          `json:"popularity"`
	VoteAverage         float64          `json:"vote_average"`
	VoteCount           int              `json:"vote_count"`
	Adult               bool             `json:"adult"`
	Video               bool             `json:"video"`
	Homepage            string           `json:"homepage"`
	PosterPath          string           `json:"poster_path"`
	BackdropPath        string           `json:"backdrop_path"`
	OriginCountry       []string         `json:"origin_country"`
	BelongsToCollection *CollectionRef   `json:"belongs_to_collection"`
	Genres              []Genre          `json:"genres"`
	ProductionCompanies []Company        `json:"production_companies"`
	ProductionCountries []Country        `json:"production_countries"`
	SpokenLanguages     []SpokenLanguage `json:"spoken_languages"`

	// append_to_response
	Credits           *Credits        `json:"credits"`
	Images            *Images         `json:"images"`
	Videos            *Videos         `json:"videos"`
	ExternalIDs       map[string]any  `json:"external_ids"`
	Keywords          *Keywords       `json:"keywords"`
	ReleaseDates      *ReleaseDates   `json:"release_dates"`
	AlternativeTitles *AltTitles      `json:"alternative_titles"`
	Translations      *Translations   `json:"translations"`
	Recommendations   *Paged          `json:"recommendations"`
	Similar           *Paged          `json:"similar"`
	WatchProviders    *WatchProviders `json:"watch/providers"`
	Reviews           *Reviews        `json:"reviews"`
	Lists             *Lists          `json:"lists"`

	Raw json.RawMessage `json:"-"`
}

type Collection struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	Overview     string            `json:"overview"`
	PosterPath   string            `json:"poster_path"`
	BackdropPath string            `json:"backdrop_path"`
	Parts        []json.RawMessage `json:"parts"`

	Raw json.RawMessage `json:"-"`
}

// ---------------------------------------------------------------- tv

type ContentRatings struct {
	Results []struct {
		Descriptors []string `json:"descriptors"`
		ISO31661    string   `json:"iso_3166_1"`
		Rating      string   `json:"rating"`
	} `json:"results"`
}

type EpisodeGroups struct {
	Results []struct {
		ID           string   `json:"id"`
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		Type         int      `json:"type"`
		GroupCount   int      `json:"group_count"`
		EpisodeCount int      `json:"episode_count"`
		Network      *Company `json:"network"`
	} `json:"results"`
}

type ScreenedTheatrically struct {
	Results []struct {
		ID            int `json:"id"`
		SeasonNumber  int `json:"season_number"`
		EpisodeNumber int `json:"episode_number"`
	} `json:"results"`
}

type Creator struct {
	ID           int    `json:"id"`
	CreditID     string `json:"credit_id"`
	Name         string `json:"name"`
	OriginalName string `json:"original_name"`
	Gender       int    `json:"gender"`
	ProfilePath  string `json:"profile_path"`
}

type IDRef struct {
	ID int `json:"id"`
}

type SeasonRef struct {
	ID           int `json:"id"`
	SeasonNumber int `json:"season_number"`
}

type TVShow struct {
	ID                  int              `json:"id"`
	Name                string           `json:"name"`
	OriginalName        string           `json:"original_name"`
	OriginalLanguage    string           `json:"original_language"`
	Overview            string           `json:"overview"`
	Tagline             string           `json:"tagline"`
	Status              string           `json:"status"`
	Type                string           `json:"type"`
	FirstAirDate        string           `json:"first_air_date"`
	LastAirDate         string           `json:"last_air_date"`
	InProduction        bool             `json:"in_production"`
	NumberOfSeasons     int              `json:"number_of_seasons"`
	NumberOfEpisodes    int              `json:"number_of_episodes"`
	EpisodeRunTime      []int            `json:"episode_run_time"`
	Languages           []string         `json:"languages"`
	OriginCountry       []string         `json:"origin_country"`
	Popularity          float64          `json:"popularity"`
	VoteAverage         float64          `json:"vote_average"`
	VoteCount           int              `json:"vote_count"`
	Adult               bool             `json:"adult"`
	Homepage            string           `json:"homepage"`
	PosterPath          string           `json:"poster_path"`
	BackdropPath        string           `json:"backdrop_path"`
	LastEpisodeToAir    *IDRef           `json:"last_episode_to_air"`
	NextEpisodeToAir    *IDRef           `json:"next_episode_to_air"`
	CreatedBy           []Creator        `json:"created_by"`
	Genres              []Genre          `json:"genres"`
	Networks            []Company        `json:"networks"`
	ProductionCompanies []Company        `json:"production_companies"`
	ProductionCountries []Country        `json:"production_countries"`
	SpokenLanguages     []SpokenLanguage `json:"spoken_languages"`
	Seasons             []SeasonRef      `json:"seasons"`

	// append_to_response
	AggregateCredits     *AggregateCredits     `json:"aggregate_credits"`
	Credits              *Credits              `json:"credits"`
	Images               *Images               `json:"images"`
	Videos               *Videos               `json:"videos"`
	ExternalIDs          map[string]any        `json:"external_ids"`
	Keywords             *Keywords             `json:"keywords"`
	ContentRatings       *ContentRatings       `json:"content_ratings"`
	AlternativeTitles    *AltTitles            `json:"alternative_titles"`
	Translations         *Translations         `json:"translations"`
	Recommendations      *Paged                `json:"recommendations"`
	Similar              *Paged                `json:"similar"`
	WatchProviders       *WatchProviders       `json:"watch/providers"`
	Reviews              *Reviews              `json:"reviews"`
	EpisodeGroups        *EpisodeGroups        `json:"episode_groups"`
	ScreenedTheatrically *ScreenedTheatrically `json:"screened_theatrically"`

	Raw json.RawMessage `json:"-"`
}

type Season struct {
	ID           int               `json:"id"`
	SeasonNumber int               `json:"season_number"`
	Name         string            `json:"name"`
	Overview     string            `json:"overview"`
	AirDate      string            `json:"air_date"`
	PosterPath   string            `json:"poster_path"`
	VoteAverage  float64           `json:"vote_average"`
	Episodes     []json.RawMessage `json:"episodes"`

	// append_to_response
	Credits          *Credits          `json:"credits"`
	AggregateCredits *AggregateCredits `json:"aggregate_credits"`
	Images           *Images           `json:"images"`
	Videos           *Videos           `json:"videos"`
	ExternalIDs      map[string]any    `json:"external_ids"`
	Translations     *Translations     `json:"translations"`

	Raw json.RawMessage `json:"-"`
}

type Episode struct {
	ID             int          `json:"id"`
	ShowID         int          `json:"show_id"`
	SeasonNumber   int          `json:"season_number"`
	EpisodeNumber  int          `json:"episode_number"`
	Name           string       `json:"name"`
	Overview       string       `json:"overview"`
	AirDate        string       `json:"air_date"`
	Runtime        int          `json:"runtime"`
	ProductionCode string       `json:"production_code"`
	EpisodeType    string       `json:"episode_type"`
	StillPath      string       `json:"still_path"`
	VoteAverage    float64      `json:"vote_average"`
	VoteCount      int          `json:"vote_count"`
	Crew           []CrewMember `json:"crew"`
	GuestStars     []CastMember `json:"guest_stars"`

	// append_to_response
	Credits      *Credits       `json:"credits"`
	Images       *Images        `json:"images"`
	Videos       *Videos        `json:"videos"`
	ExternalIDs  map[string]any `json:"external_ids"`
	Translations *Translations  `json:"translations"`

	Raw json.RawMessage `json:"-"`
}
