package bytecode

// ExceptionHandler describes a try/catch/finally block for exception handling.
type ExceptionHandler struct {
	TryStart     int // IP where try block starts
	TryEnd       int // IP where try block ends (points to PopExcept)
	CatchStart   int // IP of catch block (0 if none)
	FinallyStart int // IP of finally block (0 if none)
	CatchVarIdx  int // Local index for catch var (-1 if none)
}
