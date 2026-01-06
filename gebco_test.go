package gebco

import "testing"

func TestGebcoTifFileUnmarshal(t *testing.T) {
	cases := []struct {
		name     string
		expected GebcoTifFile
	}{
		{
			"gebco_2023_sub_ice_n0.0_s-90.0_w-90.0_e0.0.tif",
			GebcoTifFile{
				year: 2023,
				x:    1,
				y:    1,
				data: GebcoDataSubIce,
			},
		},
		{
			"gebco_2023_sub_ice_n90.0_s0.0_w-180.0_e-90.0.tif",
			GebcoTifFile{
				year: 2023,
				x:    0,
				y:    0,
				data: GebcoDataSubIce,
			},
		},
		{
			"gebco_2022_n90.0_s0.0_w-180.0_e-90.0.tif",
			GebcoTifFile{
				year: 2022,
				x:    0,
				y:    0,
				data: GebcoDataIce,
			},
		},
		{
			"gebco_2020_tid_n90.0_s0.0_w-180.0_e-90.0.tif",
			GebcoTifFile{
				year: 2020,
				x:    0,
				y:    0,
				data: GebcoDataTypeId,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var gebcoFile GebcoTifFile
			err := gebcoFile.UnmarshalText([]byte(c.name))
			if err != nil {
				t.Error(err)
			} else {
				if gebcoFile != c.expected {
					t.Errorf("expected %+v to be %+v", gebcoFile, c.expected)
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
