package opus

import "fmt"

type StereoEncodeState struct {
	pred_prev_Q13   [2]int16
	sMid            [2]int16
	sSide           [2]int16
	mid_side_amp_Q0 [4]int
	smth_width_Q14  int16
	width_prev_Q14  int16
	silent_side_len int16
	predIx          [MAX_FRAMES_PER_PACKET][][]byte
	mid_only_flags  [MAX_FRAMES_PER_PACKET]byte
}

func (s *StereoEncodeState) Reset() {
	s.pred_prev_Q13 = [2]int16{}
	s.sMid = [2]int16{}
	s.sSide = [2]int16{}
	s.mid_side_amp_Q0 = [4]int{}
	s.smth_width_Q14 = 0
	s.width_prev_Q14 = 0
	s.silent_side_len = 0
	s.predIx = [MAX_FRAMES_PER_PACKET][][]byte{}
	s.mid_only_flags = [MAX_FRAMES_PER_PACKET]byte{}
}

func (s *StereoEncodeState) PrintAllFields() {
	// 处理数组字段
	fmt.Printf("pred_prev_Q13: %v\n", s.pred_prev_Q13)
	fmt.Printf("sMid: %v\n", s.sMid)
	fmt.Printf("sSide: %v\n", s.sSide)
	fmt.Printf("mid_side_amp_Q0: %v\n", s.mid_side_amp_Q0)

	// 基本类型字段
	fmt.Printf("smth_width_Q14: %d\n", s.smth_width_Q14)
	fmt.Printf("width_prev_Q14: %d\n", s.width_prev_Q14)
	fmt.Printf("silent_side_len: %d\n", s.silent_side_len)

	// 处理三维数组 predIx
	fmt.Println("predIx:")
	if len(s.predIx) > 0 {
		for frame := 0; frame < len(s.predIx); frame++ {
			if s.predIx[frame] == nil {
				fmt.Printf("  [%d]: nil\n", frame)
				continue
			}
			fmt.Printf("  [%d]:\n", frame)
			for channel := 0; channel < len(s.predIx[frame]); channel++ {
				if s.predIx[frame][channel] == nil {
					fmt.Printf("    [%d]: nil\n", channel)
				} else {
					fmt.Printf("    [%d]: [", channel)
					for i, b := range s.predIx[frame][channel] {
						if i > 0 {
							fmt.Print(", ")
						}
						fmt.Printf("%d", b)
					}
					fmt.Println("]")
				}
			}
		}
	} else {
		fmt.Println("  nil")
	}

	// 处理 mid_only_flags 数组
	fmt.Printf("mid_only_flags: [")
	for i, b := range s.mid_only_flags {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("%d", b)
	}
	fmt.Println("]")
}
