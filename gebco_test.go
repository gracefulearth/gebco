package pixigebco

import "testing"

func TestSplitGebcoName(t *testing.T) {
	cases := []struct {
		name        string
		expectYear  int
		expectNorth int
		expectSouth int
		expectWest  int
		expectEast  int
	}{
		{"gebco_2023_sub_ice_n0.0_s-90.0_w-90.0_e0.0.tif", 2023, 0, -90, -90, 0},
		{"gebco_2023_sub_ice_n90.0_s0.0_w-180.0_e-90.0.tif", 2023, 90, 0, -180, -90},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			year, north, south, west, east, err := SplitGebcoFileName(c.name)
			if err != nil {
				t.Error(err)
			} else {
				if year != c.expectYear {
					t.Errorf("expected year %d to be %d", year, c.expectYear)
				}
				if north != c.expectNorth {
					t.Errorf("expect north %d to be %d", north, c.expectNorth)
				}
				if south != c.expectSouth {
					t.Errorf("expect south %d to be %d", south, c.expectSouth)
				}
				if east != c.expectEast {
					t.Errorf("expect east %d to be %d", east, c.expectEast)
				}
				if west != c.expectWest {
					t.Errorf("expect west %d to be %d", west, c.expectWest)
				}
			}
		})
	}
}

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
