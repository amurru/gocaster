package domain

// Repository defines the contract for data persistence.
type PodcastRepository interface {
	Save(podcast *Podcast) error
	FindAll() ([]Podcast, error)
	FindByID(id int64) (*Podcast, error)
	Delete(id int64) error

	// Episodes
	SaveEpisode(episode *Episode) error
	FindEpisodesByPodcastID(id int64) ([]Episode, error)
	FindEpisodeByID(id int64) (*Episode, error)
	DeleteEpisode(id int64) error
}
