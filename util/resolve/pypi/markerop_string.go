// Code generated by "stringer -type=markerOp -linecomment"; DO NOT EDIT.

package pypi

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[markerOpUnknown-0]
	_ = x[markerOpLessEqual-1]
	_ = x[markerOpLess-2]
	_ = x[markerOpNotEqual-3]
	_ = x[markerOpEqualEqual-4]
	_ = x[markerOpGreaterEqual-5]
	_ = x[markerOpGreater-6]
	_ = x[markerOpTildeEqual-7]
	_ = x[markerOpEqualEqualEqual-8]
	_ = x[markerOpIn-9]
	_ = x[markerOpNotIn-10]
}

const _markerOp_name = "markerOpUnknown<=<!===>=>~====innot in"

var _markerOp_index = [...]uint8{0, 15, 17, 18, 20, 22, 24, 25, 27, 30, 32, 38}

func (i markerOp) String() string {
	if i >= markerOp(len(_markerOp_index)-1) {
		return "markerOp(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _markerOp_name[_markerOp_index[i]:_markerOp_index[i+1]]
}
