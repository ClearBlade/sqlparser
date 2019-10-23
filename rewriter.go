package sqlparser

type Rewriter interface {
	ReplacementString(in string) (out string)
}
