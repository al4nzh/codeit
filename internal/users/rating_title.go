package users


func TitleForRating(rating int) string {
	switch {
	case rating < 0:
		return "Novice"
	case rating <= 999:
		return "Novice"
	case rating <= 1199:
		return "Apprentice"
	case rating <= 1399:
		return "Specialist"
	case rating <= 1599:
		return "Expert"
	case rating <= 1799:
		return "Master"
	case rating <= 1999:
		return "Grandmaster"
	default:
		return "Legend"
	}
}
