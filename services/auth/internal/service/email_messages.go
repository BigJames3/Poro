package service

import (
	"fmt"
	"time"

	"github.com/poro/auth/internal/model"
)

// emailForCode writes the French email that carries code. It names no account
// detail beyond the address it is sent to.
func emailForCode(purpose model.EmailCodePurpose, to, code string, ttl time.Duration) EmailMessage {
	minutes := int(ttl.Minutes())
	if purpose == model.EmailCodeReset {
		return EmailMessage{
			To:      to,
			Subject: "Réinitialise ton mot de passe PORO",
			Body: fmt.Sprintf("Bonjour,\n\n"+
				"Voici ton code pour choisir un nouveau mot de passe PORO : %s\n"+
				"Il expire dans %d minutes.\n\n"+
				"Une fois le mot de passe changé, tous tes appareils seront déconnectés.\n"+
				"Si tu n'as rien demandé, ignore cet email : ton mot de passe reste inchangé.\n\n"+
				"L'équipe PORO\n", code, minutes),
		}
	}
	return EmailMessage{
		To:      to,
		Subject: "Ton code de vérification PORO",
		Body: fmt.Sprintf("Bonjour,\n\n"+
			"Voici ton code pour vérifier ton adresse email sur PORO : %s\n"+
			"Il expire dans %d minutes.\n\n"+
			"Si tu n'as pas créé de compte PORO, ignore cet email.\n\n"+
			"L'équipe PORO\n", code, minutes),
	}
}
