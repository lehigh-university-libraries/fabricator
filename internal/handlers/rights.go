package handlers

import "strings"

func rightsStatementURI(value string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "IN COPYRIGHT":
		return "http://rightsstatements.org/vocab/InC/1.0/", true
	case "IN COPYRIGHT - EU ORPHAN WORK":
		return "http://rightsstatements.org/vocab/InC-OW-EU/1.0/", true
	case "IN COPYRIGHT - EDUCATIONAL USE PERMITTED":
		return "http://rightsstatements.org/vocab/InC-EDU/1.0/", true
	case "IN COPYRIGHT - NON-COMMERCIAL USE PERMITTED":
		return "http://rightsstatements.org/vocab/InC-NC/1.0/", true
	case "IN COPYRIGHT - RIGHTS-HOLDER(S) UNLOCATABLE OR UNIDENTIFIABLE":
		return "http://rightsstatements.org/vocab/InC-RUU/1.0/", true
	case "NO COPYRIGHT - CONTRACTUAL RESTRICTIONS":
		return "http://rightsstatements.org/vocab/NoC-CR/1.0/", true
	case "NO COPYRIGHT - NON-COMMERCIAL USE ONLY":
		return "http://rightsstatements.org/vocab/NoC-NC/1.0/", true
	case "NO COPYRIGHT - OTHER KNOWN LEGAL RESTRICTIONS":
		return "http://rightsstatements.org/vocab/NoC-OKLR/1.0/", true
	case "NO COPYRIGHT - UNITED STATES":
		return "http://rightsstatements.org/vocab/NoC-US/1.0/", true
	case "COPYRIGHT NOT EVALUATED":
		return "http://rightsstatements.org/vocab/CNE/1.0/", true
	case "COPYRIGHT UNDETERMINED":
		return "http://rightsstatements.org/vocab/UND/1.0/", true
	case "NO KNOWN COPYRIGHT":
		return "http://rightsstatements.org/vocab/NKC/1.0/", true
	default:
		return "", false
	}
}
