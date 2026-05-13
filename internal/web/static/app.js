(function() {
  const themeKey = 'sishc.theme';
  const langKey = 'sishc.language';
  const themes = ['default', 'alucard', 'dracula', 'catppuccin-latte', 'catppuccin-frappe', 'catppuccin-macchiato', 'catppuccin-mocha', 'gruvbox', 'solarized-light', 'solarized-dark', 'nord', 'tokyo-night', 'light', 'soft-dawn'];
  const themeAliases = {
    catppuccin: 'catppuccin-mocha',
    moss: 'gruvbox',
  };
  const languages = ['en', 'no', 'es', 'sv', 'da', 'fi', 'de', 'nl', 'fr', 'pt', 'it', 'pl'];

  const translations = {
    en: {
      'nav.settings': 'Settings',
      'dashboard.title': 'Dashboard',
      'dashboard.add_tunnel': 'Add tunnel',
      'dashboard.edit_config': 'Edit config',
      'dashboard.logs': 'Logs',
      'dashboard.tunnels': 'Tunnels',
      'dashboard.name': 'Name',
      'dashboard.state': 'State',
      'dashboard.local_host': 'Local host',
      'dashboard.local_port': 'Local port',
      'dashboard.remote': 'Remote',
      'dashboard.actions': 'Actions',
      'dashboard.start': 'Start',
      'dashboard.stop': 'Stop',
      'dashboard.edit': 'Edit',
      'dashboard.delete': 'Delete',
      'config.title': 'Global config',
      'config.raw': 'YAML',
      'config.raw_title': 'YAML',
      'config.structured': 'Structured config',
      'config.yaml': 'YAML',
      'config.raw_save': 'Save YAML',
      'config.local_host': 'Local host',
      'config.local_port': 'Local port',
      'config.local_protocol': 'Local protocol',
      'config.remote_server': 'Remote server',
      'config.remote_port': 'Remote port',
      'config.ssh_key': 'SSH key',
      'config.web_enabled': 'Web enabled',
      'config.web_listen': 'Web listen',
      'config.save': 'Save globals',
      'tunnel.add_title': 'Add tunnel',
      'tunnel.edit_title': 'Edit tunnel',
      'tunnel.name': 'Name',
      'tunnel.local_host': 'Local host',
      'tunnel.local_port': 'Local port',
      'tunnel.local_protocol': 'Local protocol',
      'tunnel.remote_server': 'Remote server',
      'tunnel.remote_port': 'Remote port',
      'tunnel.ssh_key': 'SSH key',
      'tunnel.enabled': 'Enabled',
      'tunnel.save': 'Save tunnel',
      'tunnel.cancel': 'Cancel',
      'logs.title': 'Logs',
      'logs.refresh': 'Refresh',
      'logs.follow': 'Follow',
      'logs.pause_follow': 'Pause follow',
      'settings.title': 'Settings',
      'settings.theme': 'Theme',
      'settings.language': 'Language',
      'theme.default': 'Default',
      'theme.dracula': 'Dracula',
      'theme.catppuccin-latte': 'Catppuccin Latte',
      'theme.catppuccin-frappe': 'Catppuccin Frappé',
      'theme.catppuccin-macchiato': 'Catppuccin Macchiato',
      'theme.catppuccin-mocha': 'Catppuccin Mocha',
      'theme.gruvbox': 'Gruvbox',
      'theme.solarized-light': 'Solarized Light',
      'theme.solarized-dark': 'Solarized Dark',
      'theme.nord': 'Nord',
      'theme.tokyo-night': 'Tokyo Night',
      'theme.alucard': 'Alucard',
      'theme.light': 'Light',
      'theme.soft-dawn': 'Soft Dawn',
      'status.online': 'online',
      'status.offline': 'offline',
      'status.updated': 'Updated',
      'status.service_offline': 'service offline',
      'status.service_unavailable': 'service unavailable',
      'logs.no_log_file_yet': 'No log file yet.',
      'logs.log_file_empty': 'Log file empty.',
    },
    no: {
      'nav.settings': 'Innstillinger',
      'dashboard.title': 'Oversikt',
      'dashboard.add_tunnel': 'Legg til tunnel',
      'dashboard.edit_config': 'Rediger konfig',
      'dashboard.logs': 'Logger',
      'dashboard.tunnels': 'Tunneler',
      'dashboard.name': 'Navn',
      'dashboard.state': 'Status',
      'dashboard.local_host': 'Lokal vert',
      'dashboard.local_port': 'Lokal port',
      'dashboard.remote': 'Fjernadresse',
      'dashboard.actions': 'Handlinger',
      'dashboard.start': 'Start',
      'dashboard.stop': 'Stopp',
      'dashboard.edit': 'Rediger',
      'dashboard.delete': 'Slett',
      'config.title': 'Global konfig',
      'config.raw': 'YAML',
      'config.raw_title': 'YAML',
      'config.structured': 'Strukturert konfigurasjon',
      'config.yaml': 'YAML',
      'config.raw_save': 'Lagre YAML',
      'config.local_host': 'Lokal vert',
      'config.local_port': 'Lokal port',
      'config.local_protocol': 'Lokal protokoll',
      'config.remote_server': 'Fjernserver',
      'config.remote_port': 'Fjernport',
      'config.ssh_key': 'SSH-nøkkel',
      'config.web_enabled': 'Web aktivert',
      'config.web_listen': 'Web-adresse',
      'config.save': 'Lagre globale verdier',
      'tunnel.add_title': 'Legg til tunnel',
      'tunnel.edit_title': 'Rediger tunnel',
      'tunnel.name': 'Navn',
      'tunnel.local_host': 'Lokal vert',
      'tunnel.local_port': 'Lokal port',
      'tunnel.local_protocol': 'Lokal protokoll',
      'tunnel.remote_server': 'Fjernserver',
      'tunnel.remote_port': 'Fjernport',
      'tunnel.ssh_key': 'SSH-nøkkel',
      'tunnel.enabled': 'Aktivert',
      'tunnel.save': 'Lagre tunnel',
      'tunnel.cancel': 'Avbryt',
      'logs.title': 'Logger',
      'logs.refresh': 'Oppdater',
      'logs.follow': 'Følg',
      'logs.pause_follow': 'Stopp følging',
      'settings.title': 'Innstillinger',
      'settings.theme': 'Tema',
      'settings.language': 'Språk',
      'status.online': 'online',
      'status.offline': 'offline',
      'status.updated': 'Oppdatert',
      'status.service_offline': 'tjenesten er offline',
      'status.service_unavailable': 'tjenesten er utilgjengelig',
      'logs.no_log_file_yet': 'Ingen loggfil ennå.',
      'logs.log_file_empty': 'Loggfilen er tom.',
    },
    es: {
      'nav.settings': 'Ajustes',
      'dashboard.title': 'Panel',
      'dashboard.add_tunnel': 'Añadir túnel',
      'dashboard.edit_config': 'Editar configuración',
      'dashboard.logs': 'Registros',
      'dashboard.tunnels': 'Túneles',
      'dashboard.name': 'Nombre',
      'dashboard.state': 'Estado',
      'dashboard.local_host': 'Host local',
      'dashboard.local_port': 'Puerto local',
      'dashboard.remote': 'URL remota',
      'dashboard.actions': 'Acciones',
      'dashboard.start': 'Iniciar',
      'dashboard.stop': 'Detener',
      'dashboard.edit': 'Editar',
      'dashboard.delete': 'Eliminar',
      'config.title': 'Configuración global',
      'config.raw': 'YAML',
      'config.raw_title': 'YAML',
      'config.structured': 'Configuración estructurada',
      'config.yaml': 'YAML',
      'config.raw_save': 'Guardar YAML',
      'config.local_host': 'Host local',
      'config.local_port': 'Puerto local',
      'config.local_protocol': 'Protocolo local',
      'config.remote_server': 'Servidor remoto',
      'config.remote_port': 'Puerto remoto',
      'config.ssh_key': 'Clave SSH',
      'config.web_enabled': 'Web activado',
      'config.web_listen': 'Dirección web',
      'config.save': 'Guardar valores globales',
      'tunnel.add_title': 'Añadir túnel',
      'tunnel.edit_title': 'Editar túnel',
      'tunnel.name': 'Nombre',
      'tunnel.local_host': 'Host local',
      'tunnel.local_port': 'Puerto local',
      'tunnel.local_protocol': 'Protocolo local',
      'tunnel.remote_server': 'Servidor remoto',
      'tunnel.remote_port': 'Puerto remoto',
      'tunnel.ssh_key': 'Clave SSH',
      'tunnel.enabled': 'Activado',
      'tunnel.save': 'Guardar túnel',
      'tunnel.cancel': 'Cancelar',
      'logs.title': 'Registros',
      'logs.refresh': 'Actualizar',
      'logs.follow': 'Seguir',
      'logs.pause_follow': 'Pausar seguimiento',
      'settings.title': 'Ajustes',
      'settings.theme': 'Tema',
      'settings.language': 'Idioma',
      'status.online': 'en línea',
      'status.offline': 'sin conexión',
      'status.updated': 'Actualizado',
      'status.service_offline': 'servicio sin conexión',
      'status.service_unavailable': 'servicio no disponible',
      'logs.no_log_file_yet': 'Aún no hay archivo de registro.',
      'logs.log_file_empty': 'El archivo de registro está vacío.',
    },
    sv: {
      'nav.settings': 'Inställningar',
      'dashboard.title': 'Översikt',
      'dashboard.add_tunnel': 'Lägg till tunnel',
      'dashboard.edit_config': 'Redigera konfig',
      'dashboard.logs': 'Loggar',
      'dashboard.tunnels': 'Tunnlar',
      'dashboard.name': 'Namn',
      'dashboard.state': 'Status',
      'dashboard.local_host': 'Lokal värd',
      'dashboard.local_port': 'Lokal port',
      'dashboard.remote': 'Fjärradress',
      'dashboard.actions': 'Åtgärder',
      'dashboard.start': 'Starta',
      'dashboard.stop': 'Stoppa',
      'dashboard.edit': 'Redigera',
      'dashboard.delete': 'Ta bort',
      'config.title': 'Global konfiguration',
      'config.raw': 'YAML',
      'config.local_host': 'Lokal värd',
      'config.local_port': 'Lokal port',
      'config.local_protocol': 'Lokalt protokoll',
      'config.remote_server': 'Fjärrserver',
      'config.remote_port': 'Fjärrport',
      'config.ssh_key': 'SSH-nyckel',
      'config.web_enabled': 'Webb aktiverad',
      'config.web_listen': 'Webbadress',
      'config.save': 'Spara globala värden',
      'tunnel.add_title': 'Lägg till tunnel',
      'tunnel.edit_title': 'Redigera tunnel',
      'tunnel.name': 'Namn',
      'tunnel.local_host': 'Lokal värd',
      'tunnel.local_port': 'Lokal port',
      'tunnel.local_protocol': 'Lokal protokoll',
      'tunnel.remote_server': 'Fjärrserver',
      'tunnel.remote_port': 'Fjärrport',
      'tunnel.ssh_key': 'SSH-nyckel',
      'tunnel.enabled': 'Aktiverad',
      'tunnel.save': 'Spara tunnel',
      'tunnel.cancel': 'Avbryt',
      'logs.title': 'Loggar',
      'logs.refresh': 'Uppdatera',
      'logs.follow': 'Följ',
      'logs.pause_follow': 'Pausa följning',
      'settings.title': 'Inställningar',
      'settings.theme': 'Tema',
      'settings.language': 'Språk',
      'status.online': 'online',
      'status.offline': 'offline',
      'status.updated': 'Uppdaterad',
      'status.service_offline': 'tjänsten är offline',
      'status.service_unavailable': 'tjänsten är otillgänglig',
      'logs.no_log_file_yet': 'Ingen loggfil ännu.',
      'logs.log_file_empty': 'Loggfilen är tom.',
    },
    da: {
      'nav.settings': 'Indstillinger',
      'dashboard.title': 'Oversigt',
      'dashboard.add_tunnel': 'Tilføj tunnel',
      'dashboard.edit_config': 'Rediger konfiguration',
      'dashboard.logs': 'Logfiler',
      'dashboard.tunnels': 'Tunneler',
      'dashboard.name': 'Navn',
      'dashboard.state': 'Status',
      'dashboard.local_host': 'Lokal vært',
      'dashboard.local_port': 'Lokal port',
      'dashboard.remote': 'Fjernadresse',
      'dashboard.actions': 'Handlinger',
      'dashboard.start': 'Start',
      'dashboard.stop': 'Stop',
      'dashboard.edit': 'Rediger',
      'dashboard.delete': 'Slet',
      'config.title': 'Global konfiguration',
      'config.raw': 'YAML',
      'config.raw_title': 'YAML',
      'config.structured': 'Struktureret konfiguration',
      'config.yaml': 'YAML',
      'config.raw_save': 'Gem YAML',
      'config.local_host': 'Lokal vært',
      'config.local_port': 'Lokal port',
      'config.local_protocol': 'Lokal protokol',
      'config.remote_server': 'Fjernserver',
      'config.remote_port': 'Fjernport',
      'config.ssh_key': 'SSH-nøgle',
      'config.web_enabled': 'Web aktiveret',
      'config.web_listen': 'Webadresse',
      'config.save': 'Gem globale værdier',
      'tunnel.add_title': 'Tilføj tunnel',
      'tunnel.edit_title': 'Rediger tunnel',
      'tunnel.name': 'Navn',
      'tunnel.local_host': 'Lokal vært',
      'tunnel.local_port': 'Lokal port',
      'tunnel.local_protocol': 'Lokal protokol',
      'tunnel.remote_server': 'Fjernserver',
      'tunnel.remote_port': 'Fjernport',
      'tunnel.ssh_key': 'SSH-nøgle',
      'tunnel.enabled': 'Aktiveret',
      'tunnel.save': 'Gem tunnel',
      'tunnel.cancel': 'Annuller',
      'logs.title': 'Logfiler',
      'logs.refresh': 'Opdater',
      'logs.follow': 'Følg',
      'logs.pause_follow': 'Sæt følgning på pause',
      'settings.title': 'Indstillinger',
      'settings.theme': 'Tema',
      'settings.language': 'Sprog',
      'status.online': 'online',
      'status.offline': 'offline',
      'status.updated': 'Opdateret',
      'status.service_offline': 'tjenesten er offline',
      'status.service_unavailable': 'tjenesten er utilgængelig',
      'logs.no_log_file_yet': 'Ingen logfil endnu.',
      'logs.log_file_empty': 'Logfilen er tom.',
    },
    fi: {
      'nav.settings': 'Asetukset',
      'dashboard.title': 'Yleiskuva',
      'dashboard.add_tunnel': 'Lisää tunneli',
      'dashboard.edit_config': 'Muokkaa asetuksia',
      'dashboard.logs': 'Lokit',
      'dashboard.tunnels': 'Tunnelit',
      'dashboard.name': 'Nimi',
      'dashboard.state': 'Tila',
      'dashboard.local_host': 'Paikallinen isäntä',
      'dashboard.local_port': 'Paikallinen portti',
      'dashboard.remote': 'Etäosoite',
      'dashboard.actions': 'Toiminnot',
      'dashboard.start': 'Käynnistä',
      'dashboard.stop': 'Pysäytä',
      'dashboard.edit': 'Muokkaa',
      'dashboard.delete': 'Poista',
      'config.title': 'Yleisasetukset',
      'config.raw': 'YAML',
      'config.raw_title': 'YAML',
      'config.structured': 'Rakenteinen konfiguraatio',
      'config.yaml': 'YAML',
      'config.raw_save': 'Tallenna YAML',
      'config.local_host': 'Paikallinen isäntä',
      'config.local_port': 'Paikallinen portti',
      'config.local_protocol': 'Paikallinen protokolla',
      'config.remote_server': 'Etäpalvelin',
      'config.remote_port': 'Etäportti',
      'config.ssh_key': 'SSH-avain',
      'config.web_enabled': 'Web käytössä',
      'config.web_listen': 'Web-osoite',
      'config.save': 'Tallenna globaalit arvot',
      'tunnel.add_title': 'Lisää tunneli',
      'tunnel.edit_title': 'Muokkaa tunnelia',
      'tunnel.name': 'Nimi',
      'tunnel.local_host': 'Paikallinen isäntä',
      'tunnel.local_port': 'Paikallinen portti',
      'tunnel.local_protocol': 'Paikallinen protokolla',
      'tunnel.remote_server': 'Etäpalvelin',
      'tunnel.remote_port': 'Etäportti',
      'tunnel.ssh_key': 'SSH-avain',
      'tunnel.enabled': 'Käytössä',
      'tunnel.save': 'Tallenna tunneli',
      'tunnel.cancel': 'Peruuta',
      'logs.title': 'Lokit',
      'logs.refresh': 'Päivitä',
      'logs.follow': 'Seuraa',
      'logs.pause_follow': 'Keskeytä seuranta',
      'settings.title': 'Asetukset',
      'settings.theme': 'Teema',
      'settings.language': 'Kieli',
      'status.online': 'verkossa',
      'status.offline': 'offline',
      'status.updated': 'Päivitetty',
      'status.service_offline': 'palvelu offline',
      'status.service_unavailable': 'palvelu ei käytettävissä',
      'logs.no_log_file_yet': 'Lokitiedostoa ei vielä ole.',
      'logs.log_file_empty': 'Lokitiedosto on tyhjä.',
    },
    de: {
      'nav.settings': 'Einstellungen',
      'dashboard.title': 'Übersicht',
      'dashboard.add_tunnel': 'Tunnel hinzufügen',
      'dashboard.edit_config': 'Konfiguration bearbeiten',
      'dashboard.logs': 'Protokolle',
      'dashboard.tunnels': 'Tunnel',
      'dashboard.name': 'Name',
      'dashboard.state': 'Status',
      'dashboard.local_host': 'Lokaler Host',
      'dashboard.local_port': 'Lokaler Port',
      'dashboard.remote': 'Externe URL',
      'dashboard.actions': 'Aktionen',
      'dashboard.start': 'Starten',
      'dashboard.stop': 'Stoppen',
      'dashboard.edit': 'Bearbeiten',
      'dashboard.delete': 'Löschen',
      'config.title': 'Globale Konfiguration',
      'config.raw': 'YAML',
      'config.raw_title': 'YAML',
      'config.structured': 'Strukturierte Konfiguration',
      'config.yaml': 'YAML',
      'config.raw_save': 'YAML speichern',
      'config.local_host': 'Lokaler Host',
      'config.local_port': 'Lokaler Port',
      'config.local_protocol': 'Lokales Protokoll',
      'config.remote_server': 'Remote-Server',
      'config.remote_port': 'Remote-Port',
      'config.ssh_key': 'SSH-Schlüssel',
      'config.web_enabled': 'Web aktiviert',
      'config.web_listen': 'Webadresse',
      'config.save': 'Globale Werte speichern',
      'tunnel.add_title': 'Tunnel hinzufügen',
      'tunnel.edit_title': 'Tunnel bearbeiten',
      'tunnel.name': 'Name',
      'tunnel.local_host': 'Lokaler Host',
      'tunnel.local_port': 'Lokaler Port',
      'tunnel.local_protocol': 'Lokales Protokoll',
      'tunnel.remote_server': 'Remote-Server',
      'tunnel.remote_port': 'Remote-Port',
      'tunnel.ssh_key': 'SSH-Schlüssel',
      'tunnel.enabled': 'Aktiviert',
      'tunnel.save': 'Tunnel speichern',
      'tunnel.cancel': 'Abbrechen',
      'logs.title': 'Protokolle',
      'logs.refresh': 'Aktualisieren',
      'logs.follow': 'Folgen',
      'logs.pause_follow': 'Verfolgung pausieren',
      'settings.title': 'Einstellungen',
      'settings.theme': 'Design',
      'settings.language': 'Sprache',
      'status.online': 'online',
      'status.offline': 'offline',
      'status.updated': 'Aktualisiert',
      'status.service_offline': 'Dienst offline',
      'status.service_unavailable': 'Dienst nicht verfügbar',
      'logs.no_log_file_yet': 'Noch keine Protokolldatei.',
      'logs.log_file_empty': 'Protokolldatei ist leer.',
    },
    nl: {
      'nav.settings': 'Instellingen',
      'dashboard.title': 'Overzicht',
      'dashboard.add_tunnel': 'Tunnel toevoegen',
      'dashboard.edit_config': 'Configuratie bewerken',
      'dashboard.logs': 'Logs',
      'dashboard.tunnels': 'Tunnels',
      'dashboard.name': 'Naam',
      'dashboard.state': 'Status',
      'dashboard.local_host': 'Lokale host',
      'dashboard.local_port': 'Lokale poort',
      'dashboard.remote': 'Extern adres',
      'dashboard.actions': 'Acties',
      'dashboard.start': 'Starten',
      'dashboard.stop': 'Stoppen',
      'dashboard.edit': 'Bewerken',
      'dashboard.delete': 'Verwijderen',
      'config.title': 'Globale configuratie',
      'config.raw': 'YAML',
      'config.raw_title': 'YAML',
      'config.structured': 'Gestructureerde configuratie',
      'config.yaml': 'YAML',
      'config.raw_save': 'YAML opslaan',
      'config.local_host': 'Lokale host',
      'config.local_port': 'Lokale poort',
      'config.local_protocol': 'Lokaal protocol',
      'config.remote_server': 'Externe server',
      'config.remote_port': 'Externe poort',
      'config.ssh_key': 'SSH-sleutel',
      'config.web_enabled': 'Web ingeschakeld',
      'config.web_listen': 'Webadres',
      'config.save': 'Globale waarden opslaan',
      'tunnel.add_title': 'Tunnel toevoegen',
      'tunnel.edit_title': 'Tunnel bewerken',
      'tunnel.name': 'Naam',
      'tunnel.local_host': 'Lokale host',
      'tunnel.local_port': 'Lokale poort',
      'tunnel.local_protocol': 'Lokaal protocol',
      'tunnel.remote_server': 'Externe server',
      'tunnel.remote_port': 'Externe poort',
      'tunnel.ssh_key': 'SSH-sleutel',
      'tunnel.enabled': 'Ingeschakeld',
      'tunnel.save': 'Tunnel opslaan',
      'tunnel.cancel': 'Annuleren',
      'logs.title': 'Logs',
      'logs.refresh': 'Vernieuwen',
      'logs.follow': 'Volgen',
      'logs.pause_follow': 'Volgen pauzeren',
      'settings.title': 'Instellingen',
      'settings.theme': 'Thema',
      'settings.language': 'Taal',
      'status.online': 'online',
      'status.offline': 'offline',
      'status.updated': 'Bijgewerkt',
      'status.service_offline': 'dienst offline',
      'status.service_unavailable': 'dienst niet beschikbaar',
      'logs.no_log_file_yet': 'Nog geen logbestand.',
      'logs.log_file_empty': 'Logbestand is leeg.',
    },
    fr: {
      'nav.settings': 'Paramètres',
      'dashboard.title': 'Tableau de bord',
      'dashboard.add_tunnel': 'Ajouter un tunnel',
      'dashboard.edit_config': 'Modifier la configuration',
      'dashboard.logs': 'Journaux',
      'dashboard.tunnels': 'Tunnels',
      'dashboard.name': 'Nom',
      'dashboard.state': 'État',
      'dashboard.local_host': 'Hôte local',
      'dashboard.local_port': 'Port local',
      'dashboard.remote': 'URL distante',
      'dashboard.actions': 'Actions',
      'dashboard.start': 'Démarrer',
      'dashboard.stop': 'Arrêter',
      'dashboard.edit': 'Modifier',
      'dashboard.delete': 'Supprimer',
      'config.title': 'Configuration globale',
      'config.raw': 'YAML',
      'config.raw_title': 'YAML',
      'config.structured': 'Configuration structurée',
      'config.yaml': 'YAML',
      'config.raw_save': 'Enregistrer le YAML',
      'config.local_host': 'Hôte local',
      'config.local_port': 'Port local',
      'config.local_protocol': 'Protocole local',
      'config.remote_server': 'Serveur distant',
      'config.remote_port': 'Port distant',
      'config.ssh_key': 'Clé SSH',
      'config.web_enabled': 'Web activé',
      'config.web_listen': 'Adresse web',
      'config.save': 'Enregistrer les valeurs globales',
      'tunnel.add_title': 'Ajouter un tunnel',
      'tunnel.edit_title': 'Modifier le tunnel',
      'tunnel.name': 'Nom',
      'tunnel.local_host': 'Hôte local',
      'tunnel.local_port': 'Port local',
      'tunnel.local_protocol': 'Protocole local',
      'tunnel.remote_server': 'Serveur distant',
      'tunnel.remote_port': 'Port distant',
      'tunnel.ssh_key': 'Clé SSH',
      'tunnel.enabled': 'Activé',
      'tunnel.save': 'Enregistrer le tunnel',
      'tunnel.cancel': 'Annuler',
      'logs.title': 'Journaux',
      'logs.refresh': 'Actualiser',
      'logs.follow': 'Suivre',
      'logs.pause_follow': 'Mettre en pause',
      'settings.title': 'Paramètres',
      'settings.theme': 'Thème',
      'settings.language': 'Langue',
      'status.online': 'en ligne',
      'status.offline': 'hors ligne',
      'status.updated': 'Mis à jour',
      'status.service_offline': 'service hors ligne',
      'status.service_unavailable': 'service indisponible',
      'logs.no_log_file_yet': 'Pas encore de fichier journal.',
      'logs.log_file_empty': 'Le fichier journal est vide.',
    },
    pt: {
      'nav.settings': 'Configurações',
      'dashboard.title': 'Painel',
      'dashboard.add_tunnel': 'Adicionar túnel',
      'dashboard.edit_config': 'Editar configuração',
      'dashboard.logs': 'Registos',
      'dashboard.tunnels': 'Túneis',
      'dashboard.name': 'Nome',
      'dashboard.state': 'Estado',
      'dashboard.local_host': 'Host local',
      'dashboard.local_port': 'Porta local',
      'dashboard.remote': 'URL remota',
      'dashboard.actions': 'Ações',
      'dashboard.start': 'Iniciar',
      'dashboard.stop': 'Parar',
      'dashboard.edit': 'Editar',
      'dashboard.delete': 'Eliminar',
      'config.title': 'Configuração global',
      'config.raw': 'YAML',
      'config.raw_title': 'YAML',
      'config.structured': 'Configuração estruturada',
      'config.yaml': 'YAML',
      'config.raw_save': 'Guardar YAML',
      'config.local_host': 'Host local',
      'config.local_port': 'Porta local',
      'config.local_protocol': 'Protocolo local',
      'config.remote_server': 'Servidor remoto',
      'config.remote_port': 'Porta remota',
      'config.ssh_key': 'Chave SSH',
      'config.web_enabled': 'Web ativado',
      'config.web_listen': 'Endereço web',
      'config.save': 'Guardar valores globais',
      'tunnel.add_title': 'Adicionar túnel',
      'tunnel.edit_title': 'Editar túnel',
      'tunnel.name': 'Nome',
      'tunnel.local_host': 'Host local',
      'tunnel.local_port': 'Porta local',
      'tunnel.local_protocol': 'Protocolo local',
      'tunnel.remote_server': 'Servidor remoto',
      'tunnel.remote_port': 'Porta remota',
      'tunnel.ssh_key': 'Chave SSH',
      'tunnel.enabled': 'Ativado',
      'tunnel.save': 'Guardar túnel',
      'tunnel.cancel': 'Cancelar',
      'logs.title': 'Registos',
      'logs.refresh': 'Atualizar',
      'logs.follow': 'Seguir',
      'logs.pause_follow': 'Pausar seguimento',
      'settings.title': 'Configurações',
      'settings.theme': 'Tema',
      'settings.language': 'Idioma',
      'status.online': 'online',
      'status.offline': 'offline',
      'status.updated': 'Atualizado',
      'status.service_offline': 'serviço offline',
      'status.service_unavailable': 'serviço indisponível',
      'logs.no_log_file_yet': 'Ainda não há ficheiro de logs.',
      'logs.log_file_empty': 'O ficheiro de logs está vazio.',
    },
    it: {
      'nav.settings': 'Impostazioni',
      'dashboard.title': 'Panoramica',
      'dashboard.add_tunnel': 'Aggiungi tunnel',
      'dashboard.edit_config': 'Modifica configurazione',
      'dashboard.logs': 'Log',
      'dashboard.tunnels': 'Tunnel',
      'dashboard.name': 'Nome',
      'dashboard.state': 'Stato',
      'dashboard.local_host': 'Host locale',
      'dashboard.local_port': 'Porta locale',
      'dashboard.remote': 'URL remota',
      'dashboard.actions': 'Azioni',
      'dashboard.start': 'Avvia',
      'dashboard.stop': 'Ferma',
      'dashboard.edit': 'Modifica',
      'dashboard.delete': 'Elimina',
      'config.title': 'Configurazione globale',
      'config.raw': 'YAML',
      'config.raw_title': 'YAML',
      'config.structured': 'Configurazione strutturata',
      'config.yaml': 'YAML',
      'config.raw_save': 'Salva YAML',
      'config.local_host': 'Host locale',
      'config.local_port': 'Porta locale',
      'config.local_protocol': 'Protocollo locale',
      'config.remote_server': 'Server remoto',
      'config.remote_port': 'Porta remota',
      'config.ssh_key': 'Chiave SSH',
      'config.web_enabled': 'Web attivo',
      'config.web_listen': 'Indirizzo web',
      'config.save': 'Salva valori globali',
      'tunnel.add_title': 'Aggiungi tunnel',
      'tunnel.edit_title': 'Modifica tunnel',
      'tunnel.name': 'Nome',
      'tunnel.local_host': 'Host locale',
      'tunnel.local_port': 'Porta locale',
      'tunnel.local_protocol': 'Protocollo locale',
      'tunnel.remote_server': 'Server remoto',
      'tunnel.remote_port': 'Porta remota',
      'tunnel.ssh_key': 'Chiave SSH',
      'tunnel.enabled': 'Attivo',
      'tunnel.save': 'Salva tunnel',
      'tunnel.cancel': 'Annulla',
      'logs.title': 'Log',
      'logs.refresh': 'Aggiorna',
      'logs.follow': 'Segui',
      'logs.pause_follow': 'Metti in pausa',
      'settings.title': 'Impostazioni',
      'settings.theme': 'Tema',
      'settings.language': 'Lingua',
      'status.online': 'online',
      'status.offline': 'offline',
      'status.updated': 'Aggiornato',
      'status.service_offline': 'servizio offline',
      'status.service_unavailable': 'servizio non disponibile',
      'logs.no_log_file_yet': 'Nessun file di log ancora.',
      'logs.log_file_empty': 'Il file di log è vuoto.',
    },
    pl: {
      'nav.settings': 'Ustawienia',
      'dashboard.title': 'Panel',
      'dashboard.add_tunnel': 'Dodaj tunel',
      'dashboard.edit_config': 'Edytuj konfigurację',
      'dashboard.logs': 'Logi',
      'dashboard.tunnels': 'Tunele',
      'dashboard.name': 'Nazwa',
      'dashboard.state': 'Stan',
      'dashboard.local_host': 'Host lokalny',
      'dashboard.local_port': 'Port lokalny',
      'dashboard.remote': 'Adres zdalny',
      'dashboard.actions': 'Akcje',
      'dashboard.start': 'Uruchom',
      'dashboard.stop': 'Zatrzymaj',
      'dashboard.edit': 'Edytuj',
      'dashboard.delete': 'Usuń',
      'config.title': 'Konfiguracja globalna',
      'config.raw': 'YAML',
      'config.raw_title': 'YAML',
      'config.structured': 'Konfiguracja strukturalna',
      'config.yaml': 'YAML',
      'config.raw_save': 'Zapisz YAML',
      'config.local_host': 'Host lokalny',
      'config.local_port': 'Port lokalny',
      'config.local_protocol': 'Protokół lokalny',
      'config.remote_server': 'Serwer zdalny',
      'config.remote_port': 'Port zdalny',
      'config.ssh_key': 'Klucz SSH',
      'config.web_enabled': 'Web włączony',
      'config.web_listen': 'Adres web',
      'config.save': 'Zapisz wartości globalne',
      'tunnel.add_title': 'Dodaj tunel',
      'tunnel.edit_title': 'Edytuj tunel',
      'tunnel.name': 'Nazwa',
      'tunnel.local_host': 'Host lokalny',
      'tunnel.local_port': 'Port lokalny',
      'tunnel.local_protocol': 'Protokół lokalny',
      'tunnel.remote_server': 'Serwer zdalny',
      'tunnel.remote_port': 'Port zdalny',
      'tunnel.ssh_key': 'Klucz SSH',
      'tunnel.enabled': 'Włączony',
      'tunnel.save': 'Zapisz tunel',
      'tunnel.cancel': 'Anuluj',
      'logs.title': 'Logi',
      'logs.refresh': 'Odśwież',
      'logs.follow': 'Śledź',
      'logs.pause_follow': 'Wstrzymaj śledzenie',
      'settings.title': 'Ustawienia',
      'settings.theme': 'Motyw',
      'settings.language': 'Język',
      'theme.default': 'Domyślny',
      'theme.dracula': 'Dracula',
      'theme.catppuccin-latte': 'Catppuccin Latte',
      'theme.catppuccin-frappe': 'Catppuccin Frappé',
      'theme.catppuccin-macchiato': 'Catppuccin Macchiato',
      'theme.catppuccin-mocha': 'Catppuccin Mocha',
      'theme.gruvbox': 'Gruvbox',
      'theme.solarized-light': 'Solarized Light',
      'theme.solarized-dark': 'Solarized Dark',
      'theme.nord': 'Nord',
      'theme.tokyo-night': 'Tokyo Night',
      'theme.alucard': 'Alucard',
      'theme.light': 'Jasny',
      'theme.soft-dawn': 'Soft Dawn',
      'status.online': 'online',
      'status.offline': 'offline',
      'status.updated': 'Zaktualizowano',
      'status.service_offline': 'usługa offline',
      'status.service_unavailable': 'usługa niedostępna',
      'logs.no_log_file_yet': 'Brak pliku logów.',
      'logs.log_file_empty': 'Plik logów jest pusty.',
    },
  };

  function getTheme() {
    return normalizeTheme(localStorage.getItem(themeKey) || 'default');
  }

  function normalizeTheme(theme) {
    const value = themeAliases[theme] || theme;
    return themes.includes(value) ? value : 'default';
  }

  function setTheme(theme) {
    const value = normalizeTheme(theme);
    document.documentElement.setAttribute('data-theme', value);
    localStorage.setItem(themeKey, value);
    return value;
  }

  function getLanguage() {
    return localStorage.getItem(langKey) || 'en';
  }

  function setLanguage(language) {
    const value = languages.includes(language) ? language : 'en';
    document.documentElement.setAttribute('lang', value);
    localStorage.setItem(langKey, value);
    translateDocument();
    return value;
  }

  function translate(key) {
    const lang = getLanguage();
    return (translations[lang] && translations[lang][key]) || translations.en[key] || key;
  }

  function translateDocument(root) {
    const scope = root || document;
    scope.querySelectorAll('[data-i18n]').forEach(function(el) {
      const key = el.getAttribute('data-i18n');
      const text = translate(key);
      if (text) {
        el.textContent = text;
      }
    });
    scope.querySelectorAll('[data-i18n-placeholder]').forEach(function(el) {
      const key = el.getAttribute('data-i18n-placeholder');
      const text = translate(key);
      if (text) {
        el.setAttribute('placeholder', text);
      }
    });
    scope.querySelectorAll('[data-i18n-follow]').forEach(function(el) {
      const key = el.getAttribute('data-i18n-follow');
      const text = translate(key);
      if (text) {
        el.textContent = text;
      }
    });
  }

  function bindSettingsPage() {
    const themeSelect = document.getElementById('theme');
    const languageSelect = document.getElementById('language');
    if (!themeSelect || !languageSelect) {
      return;
    }

    themeSelect.value = getTheme();
    languageSelect.value = languages.includes(getLanguage()) ? getLanguage() : 'en';
    setTheme(themeSelect.value);
    setLanguage(languageSelect.value);

    themeSelect.addEventListener('change', function() {
      setTheme(themeSelect.value);
    });
    languageSelect.addEventListener('change', function() {
      setLanguage(languageSelect.value);
    });
  }

  function bindDashboard() {
    const banner = document.getElementById('daemon-state');
    const stamp = document.getElementById('status-updated');
    const table = document.getElementById('tunnels-table');
    if (!banner || !stamp || !table) {
      return;
    }

    async function refreshStatus() {
      const t = translate;
      try {
        const res = await fetch('/api/status', { cache: 'no-store' });
        const data = await res.json();
        if (!data.ok) {
          banner.textContent = t('status.offline');
          banner.className = 'badge bad';
          stamp.textContent = data.error || t('status.service_offline');
          return;
        }
        banner.textContent = t('status.online');
        banner.className = 'badge ok';
        stamp.textContent = t('status.updated') + ' ' + new Date().toLocaleTimeString();
        const rows = new Map((data.statuses || []).map(st => [st.name, st]));
        document.querySelectorAll('#tunnels-table tbody tr[data-name]').forEach(row => {
          const name = row.getAttribute('data-name');
          const st = rows.get(name);
          if (!st) return;
          const state = row.querySelector('[data-field="state"]');
          const host = row.querySelector('[data-field="host"]');
          const port = row.querySelector('[data-field="port"]');
          const remote = row.querySelector('[data-field="remote"]');
          state.textContent = st.state || '-';
          state.className = 'badge ' + ((st.state === 'running') ? 'ok' : (st.state === 'disabled' ? 'muted' : (st.state === 'error' ? 'bad' : 'warn')));
          host.textContent = st.local_host || '-';
          port.textContent = st.local_port || '-';
          remote.textContent = st.remote || '-';
        });
      } catch (err) {
        banner.textContent = t('status.offline');
        banner.className = 'badge bad';
        stamp.textContent = t('status.service_unavailable');
      }
    }

    refreshStatus();
    setInterval(refreshStatus, 5000);
  }

  function bindLogs() {
    const box = document.getElementById('log-output');
    const src = document.getElementById('log-stream-source');
    const followMeta = document.getElementById('log-follow-label');
    if (!box || !src) {
      return;
    }
    const ev = new EventSource(src.getAttribute('data-src'));
    ev.addEventListener('line', function(line) {
      box.insertAdjacentHTML('afterbegin', line.data + '\n');
    });
    if (followMeta) {
      followMeta.textContent = translate(followMeta.getAttribute('data-i18n-follow'));
    }
  }

  function bootstrap() {
    setTheme(getTheme());
    setLanguage(getLanguage());
    translateDocument();
    bindSettingsPage();
    bindDashboard();
    bindLogs();
  }

  window.sishcI18n = {
    t: translate,
    setTheme: setTheme,
    setLanguage: setLanguage,
    translateDocument: translateDocument,
  };

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', bootstrap);
  } else {
    bootstrap();
  }
})();
