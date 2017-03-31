package bbr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimate(t *testing.T) {
	s := newStats()
	est := s.estABE()
	assert.EqualValues(t, 0, est)
	ignored := 0
	for i, d := range data {
		bytes := d[0] * 1024
		s.update(bytes, d[1])
		if bytes < minBytesThreshold {
			ignored++
		}
		assert.True(t, s.size <= limit)
		if i-ignored >= limit-1 {
			assert.Equal(t, limit, s.size)
		}
		est = s.estABE()
		t.Log(est)
	}
}

// This data was taken from real-world measurements. It contains kb_sent and ABE
// as reported by BBR.
var data = [][]float64{
	[]float64{1.1, 0.058688000000000004},
	[]float64{6.8, 0.29344},
	[]float64{6.7, 0.29206400000000005},
	[]float64{3.1, 0.176064},
	[]float64{6.9, 0.302416},
	[]float64{6.6, 0.302416},
	[]float64{1.2, 0.060064},
	[]float64{7.1, 0.302416},
	[]float64{7.5, 0.361104},
	[]float64{3.4, 0.175376},
	[]float64{7.8, 0.350752},
	[]float64{2.8, 0.120824},
	[]float64{5.6, 0.241656},
	[]float64{7.5, 0.36456},
	[]float64{8.1, 0.360416},
	[]float64{3.9, 0.176064},
	[]float64{4.5, 0.234064},
	[]float64{7.1, 0.302416},
	[]float64{2.0, 0.11737600000000001},
	[]float64{3.0, 0.175376},
	[]float64{2.1, 0.11737600000000001},
	[]float64{2.8, 0.120824},
	[]float64{8.0, 0.35144},
	[]float64{5.3, 0.233368},
	[]float64{1.3, 0.058688000000000004},
	[]float64{6.9, 0.29344},
	[]float64{7.1, 0.30104000000000003},
	[]float64{1.0, 0.060064},
	[]float64{2.4, 0.11737600000000001},
	[]float64{1.3, 0.060064},
	[]float64{3.8, 0.18228},
	[]float64{5.0, 0.233368},
	[]float64{1.2, 0.058688000000000004},
	[]float64{22, 0.637984},
	[]float64{99, 2.032712},
	[]float64{3.2, 0.180208},
	[]float64{21, 0.641432},
	[]float64{25, 0.644192},
	[]float64{1.7, 0.10840000000000001},
	[]float64{1.9, 0.12151999999999999},
	[]float64{2.0, 0.12151999999999999},
	[]float64{4.2, 0.18089599999999997},
	[]float64{1.9, 0.073184},
	[]float64{5.1, 0.14775200000000002},
	[]float64{7.9, 0.220256},
	[]float64{4.9, 0.146376},
	[]float64{8.0, 0.22370400000000001},
	[]float64{3.9, 0.10908799999999999},
	[]float64{4.9, 0.155352},
	[]float64{3.2, 0.115304},
	[]float64{1.3, 0.058688000000000004},
	[]float64{4.1, 0.11254399999999999},
	[]float64{7.0, 0.302416},
	[]float64{3.7, 0.178136},
	[]float64{6.9, 0.301728},
	[]float64{7.7, 0.350056},
	[]float64{7.9, 0.34868},
	[]float64{4.1, 0.175376},
	[]float64{4.6, 0.23268},
	[]float64{3.6, 0.18089599999999997},
	[]float64{7.5, 0.34868},
	[]float64{5.6, 0.241656},
	[]float64{6.8, 0.291368},
	[]float64{3.8, 0.18228},
	[]float64{3.9, 0.181584},
	[]float64{4.1, 0.17468},
	[]float64{1.1, 0.058688000000000004},
	[]float64{7.5, 0.363176},
	[]float64{4.9, 0.240968},
	[]float64{8.2, 0.349368},
	[]float64{7.3, 0.3618},
	[]float64{1.1, 0.060064},
	[]float64{6.4, 0.29344},
	[]float64{5.3, 0.233368},
	[]float64{6.6, 0.292752},
	[]float64{5.4, 0.233368},
	[]float64{7.1, 0.292752},
	[]float64{7.7, 0.350752},
	[]float64{5.4, 0.240968},
	[]float64{3.3, 0.181584},
	[]float64{4.2, 0.17468},
	[]float64{6.4, 0.292752},
	[]float64{4.6, 0.23544},
	[]float64{1.9, 0.11806399999999999},
	[]float64{4.2, 0.176064},
	[]float64{7.6, 0.349368},
	[]float64{5.6, 0.231992},
	[]float64{1.4, 0.058688000000000004},
	[]float64{5.7, 0.23475200000000002},
	[]float64{3.6, 0.17399199999999998},
	[]float64{5.4, 0.233368},
	[]float64{7.3, 0.350056},
	[]float64{6.9, 0.292752},
	[]float64{3.7, 0.175376},
	[]float64{5.3, 0.234064},
	[]float64{16, 0.46122399999999997},
	[]float64{5.0, 0.23475200000000002},
	[]float64{3.6, 0.175376},
	[]float64{4.9, 0.233368},
	[]float64{3.2, 0.17468},
	[]float64{7.2, 0.29344},
	[]float64{2.0, 0.11737600000000001},
	[]float64{4.5, 0.234064},
	[]float64{3.0, 0.17468},
	[]float64{2.0, 0.11668},
	[]float64{3.4, 0.17399199999999998},
	[]float64{49, 0.941096},
	[]float64{22, 0.421176},
	[]float64{6.5, 0.292752},
	[]float64{21, 0.6559360000000001},
	[]float64{20, 0.640744},
	[]float64{4.1, 0.175376},
	[]float64{1.4, 0.057991999999999995},
	[]float64{7.2, 0.29482400000000003},
	[]float64{2.6, 0.11668},
	[]float64{6.1, 0.292752},
	[]float64{4.7, 0.234064},
	[]float64{7.8, 0.34868},
	[]float64{22, 0.638672},
	[]float64{2.5, 0.120824},
	[]float64{24, 0.666288},
	[]float64{5.7, 0.242344},
	[]float64{1.6, 0.11737600000000001},
	[]float64{39, 1.113712},
	[]float64{2.8, 0.11668},
	[]float64{2.0, 0.11737600000000001},
	[]float64{2.0, 0.11806399999999999},
	[]float64{6.2, 0.30104000000000003},
	[]float64{2.1, 0.12013599999999999},
	[]float64{2.9, 0.181584},
	[]float64{8.1, 0.353512},
	[]float64{2.6, 0.120824},
	[]float64{6.2, 0.292752},
	[]float64{3.8, 0.176064},
	[]float64{7.0, 0.292752},
	[]float64{6.7, 0.301728},
	[]float64{3.8, 0.18228},
	[]float64{3.6, 0.175376},
	[]float64{25, 0.6400560000000001},
	[]float64{21, 0.638672},
	[]float64{7.5, 0.363176},
	[]float64{4.1, 0.181584},
	[]float64{4.5, 0.241656},
	[]float64{17, 0.424632},
	[]float64{3.4, 0.176064},
	[]float64{5.1, 0.234064},
	[]float64{4.2, 0.175376},
	[]float64{2.3, 0.11737600000000001},
	[]float64{3.3, 0.176064},
	[]float64{1.5, 0.12013599999999999},
	[]float64{2.8, 0.11737600000000001},
	[]float64{6.5, 0.29344},
	[]float64{3.3, 0.175376},
	[]float64{1.5, 0.11737600000000001},
	[]float64{1.0, 0.060064},
	[]float64{18, 0.644192},
	[]float64{4.1, 0.175376},
	[]float64{2.6, 0.11737600000000001},
	[]float64{8.3, 0.640744},
	[]float64{5.2, 0.23475200000000002},
	[]float64{374, 6.85212},
	[]float64{22, 0.659384},
	[]float64{1.5, 0.120824},
	[]float64{1.7, 0.120824},
	[]float64{2.2, 0.12151999999999999},
	[]float64{21, 0.661456},
	[]float64{25, 0.6055280000000001},
	[]float64{18, 0.604152},
	[]float64{20, 0.6062240000000001},
	[]float64{24, 0.6020800000000001},
	[]float64{22, 0.582744},
	[]float64{22, 0.644192},
	[]float64{26, 0.603456},
	[]float64{22, 0.6020800000000001},
	[]float64{17, 0.643504},
	[]float64{25, 0.640744},
	[]float64{22, 0.606912},
	[]float64{23, 0.582744},
	[]float64{28, 0.6400560000000001},
	[]float64{24, 0.60484},
	[]float64{19, 0.663528},
	[]float64{20, 0.641432},
	[]float64{21, 0.640744},
	[]float64{22, 0.63936},
	[]float64{20, 0.644192},
	[]float64{18, 0.663528},
	[]float64{17, 0.662144},
	[]float64{22, 0.662144},
	[]float64{20, 0.666288},
	[]float64{21, 0.660768},
	[]float64{26, 0.5862},
	[]float64{19, 0.5862},
	[]float64{16, 0.64212},
	[]float64{19, 0.643504},
	[]float64{18, 0.66284},
	[]float64{20, 0.6020800000000001},
	[]float64{19, 0.644192},
	[]float64{22, 0.659384},
	[]float64{7.6, 0.363176},
	[]float64{19, 0.654552},
	[]float64{52, 1.51556},
	[]float64{46, 1.32568},
	[]float64{24, 0.6027680000000001},
	[]float64{6.6, 0.301728},
	[]float64{23, 0.619336},
	[]float64{22, 0.506792},
	[]float64{6.0, 0.29206400000000005},
	[]float64{3.4, 0.17468},
	[]float64{1.7, 0.11668},
	[]float64{7.5, 0.35144},
	[]float64{25, 0.50196},
	[]float64{22, 0.640744},
	[]float64{17, 0.39493599999999995},
	[]float64{79, 2.208776},
	[]float64{329, 5.217112},
	[]float64{24, 0.63936},
	[]float64{22, 0.656624},
	[]float64{21, 0.63936},
	[]float64{24, 0.6586960000000001},
	[]float64{89, 2.031328},
	[]float64{36, 0.43568},
	[]float64{146, 2.84676},
	[]float64{109, 2.522936},
	[]float64{15, 0.664216},
	[]float64{25, 0.663528},
	[]float64{14, 0.661456},
	[]float64{229, 4.508008},
	[]float64{22, 0.660072},
	[]float64{24, 0.6400560000000001},
	[]float64{26, 0.64212},
	[]float64{25, 0.643504},
	[]float64{22, 0.638672},
	[]float64{21, 0.637288},
	[]float64{4.4, 0.240968},
	[]float64{75, 1.871144},
	[]float64{23, 0.64212},
	[]float64{26, 0.638672},
	[]float64{228, 4.2042079999999995},
	[]float64{21, 0.644192},
	[]float64{23, 0.642816},
	[]float64{16, 0.637984},
	[]float64{20, 0.638672},
	[]float64{5.2, 0.242344},
	[]float64{28, 0.638672},
	[]float64{27, 0.640744},
	[]float64{22, 0.638672},
	[]float64{26, 0.642816},
	[]float64{43, 0.63936},
	[]float64{20, 0.635216},
	[]float64{21, 0.6310800000000001},
	[]float64{22, 0.6338400000000001},
	[]float64{24, 0.64212},
	[]float64{18, 0.643504},
	[]float64{20, 0.640744},
	[]float64{27, 0.6400560000000001},
	[]float64{120, 2.7763359999999997},
	[]float64{29, 0.644888},
	[]float64{154, 2.278512},
	[]float64{28, 0.48056},
	[]float64{123, 1.693008},
	[]float64{26, 0.641432},
	[]float64{20, 0.6338400000000001},
	[]float64{19, 0.584816},
	[]float64{22, 0.644192},
	[]float64{25, 0.6400560000000001},
	[]float64{18, 0.642816},
	[]float64{32, 0.45156},
	[]float64{27, 0.644192},
	[]float64{28, 0.6338400000000001},
	[]float64{35, 0.82164},
	[]float64{21, 0.637984},
	[]float64{41, 0.789192},
	[]float64{3.6, 0.176064},
	[]float64{26, 0.642816},
	[]float64{27, 0.6649120000000001},
	[]float64{26, 0.6586960000000001},
	[]float64{30, 0.660768},
	[]float64{15, 0.644192},
	[]float64{21, 0.641432},
	[]float64{17, 0.6400560000000001},
	[]float64{15, 0.641432},
	[]float64{19, 0.637288},
	[]float64{1.8, 0.11668},
	[]float64{15, 0.662144},
	[]float64{2.3, 0.12151999999999999},
	[]float64{2.5, 0.120824},
	[]float64{7.7, 0.360416},
	[]float64{5.3, 0.242344},
	[]float64{3.3, 0.180208},
	[]float64{3.4, 0.175376},
	[]float64{3.1, 0.176064},
	[]float64{5.1, 0.23268},
	[]float64{7.4, 0.350752},
	[]float64{6.0, 0.29344},
	[]float64{24, 0.6338400000000001},
	[]float64{20, 0.641432},
	[]float64{10, 0.641432},
	[]float64{17, 0.662144},
	[]float64{3.1, 0.18089599999999997},
	[]float64{22, 0.638672},
	[]float64{18, 0.661456},
	[]float64{14, 0.660072},
	[]float64{2.5, 0.115304},
	[]float64{6.6, 0.289992},
	[]float64{1.1, 0.057991999999999995},
	[]float64{2.8, 0.11668},
	[]float64{2.2, 0.11737600000000001},
	[]float64{5.6, 0.233368},
	[]float64{4.8, 0.23268},
	[]float64{7.6, 0.350056},
	[]float64{6.8, 0.29482400000000003},
	[]float64{1.5, 0.11737600000000001},
	[]float64{1.7, 0.11737600000000001},
	[]float64{6.0, 0.29344},
	[]float64{7.3, 0.350752},
	[]float64{1.2, 0.058688000000000004},
	[]float64{3.2, 0.175376},
	[]float64{6.8, 0.29206400000000005},
	[]float64{4.7, 0.228536},
	[]float64{6.4, 0.29068},
	[]float64{7.2, 0.29206400000000005},
	[]float64{17, 0.641432},
	[]float64{17, 0.662144},
	[]float64{6.3, 0.292752},
	[]float64{1.7, 0.11668},
	[]float64{21, 0.64212},
	[]float64{13, 0.642816},
	[]float64{19, 0.644192},
	[]float64{26, 0.644192},
	[]float64{1400, 2.1604479999999997},
	[]float64{7.6, 0.35144},
	[]float64{18, 0.664216},
	[]float64{206, 1.804856},
	[]float64{28, 0.644192},
	[]float64{22, 0.645576},
	[]float64{30, 0.658008},
	[]float64{13, 0.372848},
	[]float64{19, 0.660768},
	[]float64{21, 0.661456},
	[]float64{19, 0.642816},
	[]float64{24, 0.6586960000000001},
	[]float64{14, 0.662144},
	[]float64{175, 1.80624},
	[]float64{23, 0.643504},
	[]float64{23, 0.643504},
	[]float64{20, 0.644192},
	[]float64{21, 0.635912},
	[]float64{21, 0.641432},
	[]float64{140, 2.763912},
	[]float64{18, 0.6586960000000001},
	[]float64{19, 0.660768},
	[]float64{33, 0.6586960000000001},
	[]float64{84, 1.980232},
	[]float64{18, 0.640744},
	[]float64{20, 0.637288},
	[]float64{20, 0.642816},
	[]float64{27, 0.63936},
	[]float64{20, 0.640744},
	[]float64{24, 0.643504},
	[]float64{24, 0.64212},
	[]float64{12, 0.635912},
	[]float64{17, 0.64212},
	[]float64{2500, 25.694784},
	[]float64{27, 0.660072},
	[]float64{11000, 53.775800000000004},
	[]float64{21, 0.637984},
	[]float64{20, 0.64212},
}
