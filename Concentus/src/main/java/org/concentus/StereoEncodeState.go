package opus
type StereoEncodeState struct {
    pred_prev_Q13      [2]int16
    sMid               [2]int16
    sSide              [2]int16
    mid_side_amp_Q0    [4]int32
    smth_width_Q14     int16
    width_prev_Q14     int16
    silent_side_len    int16
    predIx             [SilkConstants.MAX_FRAMES_PER_PACKET][2][3]byte
    mid_only_flags     [SilkConstants.MAX_FRAMES_PER_PACKET]byte
}

func (s *StereoEncodeState) Reset() {
    s.pred_prev_Q13 = [2]int16{}
    s.sMid = [2]int16{}
    s.sSide = [2]int16{}
    s.mid_side_amp_Q0 = [4]int32{}
    s.smth_width_Q14 = 0
    s.width_prev_Q14 = 0
    s.silent_side_len = 0
    s.predIx = [SilkConstants.MAX_FRAMES_PER_PACKET][2][3]byte{}
    s.mid_only_flags = [SilkConstants.MAX_FRAMES_PER_PACKET]byte{}
}