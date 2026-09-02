package configsync

import "context"

// ChannelSnapshot is the public, credential-free configuration channel.
const ChannelSnapshot = "snapshot"

func init() {
	RegisterChannel(ChannelDescriptor{
		ID:          ChannelSnapshot,
		Description: "GitHub/Gitee public configuration snapshot",
		Fields: []ChannelField{
			{Name: "URL", Prompt: "Public snapshot URL(s), space separated"},
		},
		Validate: func(context.Context, map[string]string) error { return nil },
	})
}
