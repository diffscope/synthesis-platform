package service

import bindings "diffscope-synthesis-platform/native/bindings"

func Add(a int, b int) int {
	calculator := bindings.NewCalculator(a, b)
	defer bindings.DeleteCalculator(calculator)

	return calculator.Add()
}
