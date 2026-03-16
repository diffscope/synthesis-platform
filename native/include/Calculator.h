#ifndef CALCULATOR_H
#define CALCULATOR_H

// example native code to be used in Go via SWIG
class Calculator {
public:
	explicit Calculator(int a, int b);

	int Add() const;
private:
	int m_a;
	int m_b;
};

#endif // CALCULATOR_H