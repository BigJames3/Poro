param([string]$Command = "help")

switch ($Command) {
    "up"     { docker compose up -d }
    "down"   { docker compose down }
    "logs"   { docker compose logs -f }
    "ps"     { docker compose ps }
    "clean"  { docker compose down -v }
    "help"   {
        Write-Host "Commandes disponibles :" -ForegroundColor Cyan
        Write-Host "  .\dev.ps1 up      - Démarrer les services"
        Write-Host "  .\dev.ps1 down    - Arrêter les services"
        Write-Host "  .\dev.ps1 logs    - Voir les logs"
        Write-Host "  .\dev.ps1 ps      - État des services"
        Write-Host "  .\dev.ps1 clean   - Tout supprimer"
    }
    default  { Write-Host "Commande inconnue : $Command" }
}
