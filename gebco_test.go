package pixigebco

import "testing"

func TestFromDecimal(t *testing.T) {
	tests := []struct {
		desc     string
		input    float64
		expected GebcoArc
	}{
		// Positive values
		{"0.0", 0, GebcoArc{Degree: 0, Minute: 0, Second: 0}},
		{"90", 90, GebcoArc{Degree: 90, Minute: 0, Second: 0}},
		{"180", 180, GebcoArc{Degree: 180, Minute: 0, Second: 0}},
		{"-180", -180, GebcoArc{Degree: -180, Minute: 0, Second: 0}},

		// Negative values
		{"90.5", 90.5, GebcoArc{Degree: 90, Minute: 30, Second: 0}},
		{"-90.5", -90.5, GebcoArc{Degree: -90, Minute: 30, Second: 0}},
		{"180.25", 180.25, GebcoArc{Degree: 180, Minute: 15, Second: 0}},

		// Edge cases
		{"99.9999", 99.9999, GebcoArc{Degree: 99, Minute: 59, Second: 59}},
		{"-0.0005", -0.0005, GebcoArc{Degree: 0, Minute: 0, Second: -1}},

		// Rounding errors
		{"-0.0000001", -0.0000001, GebcoArc{Degree: 0, Minute: 0, Second: 0}},
		{"0.0000001", 0.0000001, GebcoArc{Degree: 0, Minute: 0, Second: 0}},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			actual := FromDecimal(test.input)
			if !actual.Equal(test.expected) {
				t.Errorf("Expected %v, got %v", test.expected, actual)
			}
		})
	}
}
