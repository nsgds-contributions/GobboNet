package supervisor

// terminateGroup is a variable so a test can substitute a refusal. The seam
// exists because the interesting behaviour -- that a refused graceful stop is
// escalated immediately rather than waited out -- is otherwise only reachable
// on Windows, where nothing here runs.
//
// ⛔ Tests replace this. Nothing in the package's tests may call t.Parallel, or
// the swap races against whatever else is running.
var terminateGroup = terminateGroupImpl
