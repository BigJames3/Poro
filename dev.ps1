param([string]$Command = "help")

switch ($Command) {
    "up"     { docker compose up -d }
    "down"   { docker compose down }
    "logs"   { docker compose logs -f }
    "ps"     { docker compose ps }
    "clean"  {
        $answer = Read-Host "Supprimer DÉFINITIVEMENT les volumes (données locales) ? Taper 'oui' pour confirmer"
        if ($answer -eq "oui") { docker compose down -v } else { Write-Host "Annulé." }
    }
    "help"   {
        Write-Host "Commandes disponibles :" -ForegroundColor Cyan
        Write-Host "  .\dev.ps1 up      - Démarrer les services"
        Write-Host "  .\dev.ps1 down    - Arrêter les services"
        Write-Host "  .\dev.ps1 logs    - Voir les logs"
        Write-Host "  .\dev.ps1 ps      - État des services"
        Write-Host "  .\dev.ps1 clean   - Supprimer conteneurs ET volumes (confirmation demandée)"
    }
    default  { Write-Host "Commande inconnue : $Command" }
}
