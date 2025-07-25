package opus
type ChannelLayout struct {
    nb_streams         int
    nb_coupled_streams int
    nb_channels        int
    mapping            []int
}

func validate_layout(layout ChannelLayout) int {
    max_channel := layout.nb_streams + layout.nb_coupled_streams
    if max_channel > 255 {
        return 0
    }
    for i := 0; i < layout.nb_channels; i++ {
        if layout.mapping[i] >= max_channel && layout.mapping[i] != 255 {
            return 0
        }
    }
    return 1
}

func get_left_channel(layout ChannelLayout, stream_id int, prev int) int {
    start := 0
    if prev >= 0 {
        start = prev + 1
    }
    for i := start; i < layout.nb_channels; i++ {
        if layout.mapping[i] == stream_id*2 {
            return i
        }
    }
    return -1
}

func get_right_channel(layout ChannelLayout, stream_id int, prev int) int {
    start := 0
    if prev >= 0 {
        start = prev + 1
    }
    for i := start; i < layout.nb_channels; i++ {
        if layout.mapping[i] == stream_id*2+1 {
            return i
        }
    }
    return -1
}

func get_mono_channel(layout ChannelLayout, stream_id int, prev int) int {
    start := 0
    if prev >= 0 {
        start = prev + 1
    }
    for i := start; i < layout.nb_channels; i++ {
        if layout.mapping[i] == stream_id+layout.nb_coupled_streams {
            return i
        }
    }
    return -1
}