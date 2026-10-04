// Distillation shapes shared by the LLM capability that produces them, the profile capability that
// consumes them and the record layer that stores them.

package core

// EmotionScore is the VAD emotion estimate of one distillation round.
type EmotionScore struct {
	Valence   float64 `json:"valence"`
	Arousal   float64 `json:"arousal"`
	Dominance float64 `json:"dominance"`
}

// MBTIScore holds four MBTI dimensions in [-1,1] — negative = I/N/T/J, positive = E/S/F/P, magnitude =
// strength.
type MBTIScore struct {
	IE   float64 `json:"i_e"`
	NS   float64 `json:"n_s"`
	TF   float64 `json:"t_f"`
	JP   float64 `json:"j_p"`
	Type string  `json:"-"`
}

// DeriveMBTIType reads the four dimensions as one type.
func DeriveMBTIType(m MBTIScore) string {
	if m.IE == 0 && m.NS == 0 && m.TF == 0 && m.JP == 0 {
		return ""
	}
	letter := func(v float64, neg, pos byte) byte {
		switch {
		case v == 0:
			return 'X'
		case v < 0:
			return neg
		default:
			return pos
		}
	}
	return string([]byte{
		letter(m.IE, 'I', 'E'),
		letter(m.NS, 'N', 'S'),
		letter(m.TF, 'T', 'F'),
		letter(m.JP, 'J', 'P'),
	})
}

// NodeEmotion is the per-L1-node emotion signal written back after a distillation round.
type NodeEmotion struct {
	Valence float64
	Arousal float64
}

// DistillSample is one L1 node prepared for the distillation prompt: the three fields the prompt
// renders.
type DistillSample struct {
	IDHash     uint64
	Keywords   []string
	Importance float64
}
