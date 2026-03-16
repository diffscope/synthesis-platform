#include "Calculator.h"

Calculator::Calculator(int a, int b) : m_a(a), m_b(b) {
}

int Calculator::Add() const {
	return m_a + m_b;
}