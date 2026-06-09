package constant

type TaskPlatform string

const (
	TaskPlatformSuno       TaskPlatform = "suno"
	TaskPlatformMidjourney              = "mj"
	TaskPlatformSystem                  = "system"
)

const (
	SunoActionMusic  = "MUSIC"
	SunoActionLyrics = "LYRICS"

	TaskActionGenerate               = "generate"
	TaskActionTextGenerate           = "textGenerate"
	TaskActionFirstTailGenerate      = "firstTailGenerate"
	TaskActionReferenceGenerate      = "referenceGenerate"
	TaskActionRemix                  = "remixGenerate"
	TaskActionUpstreamGroupRatioSync = "upstream_group_ratio_sync"
)

var SunoModel2Action = map[string]string{
	"suno_music":  SunoActionMusic,
	"suno_lyrics": SunoActionLyrics,
}
