COOKIEKILL - WINDOWS SERVICE DEPLOYMENT

This package contains a standalone cookiekill.exe with the entire browser client
embedded, config.json, installation/removal scripts, and third-party notices.
The target server does not need Go, Node.js, a separate web server, or a service
wrapper. Use the amd64 archive for x64 Windows and the arm64 archive for ARM64.
PowerShell 5.1 or newer is required by the deployment scripts.

FIRST INSTALLATION

1. Extract the ZIP to a permanent, dedicated local directory, for example
   C:\Services\CookieKill. Keep the files together. Do not run from the ZIP or
   a network share. Use an administrator-controlled directory for the package.

2. Edit config.json next to cookiekill.exe. Fill in domain, smtp.host, and
   smtp.from. Set SMTP credentials only if your provider requires login. Example:

   {
     "dev": false,
     "addr": "127.0.0.1:8080",
     "domain": "play.example.com",
     "email": "admin@example.com",
     "dataDir": "data",
     "logFile": "logs/cookiekill.log",
     "smtp": {
       "host": "smtp.example.com",
       "port": 587,
       "user": "",
       "password": "",
       "from": "game@example.com"
     }
   }

   email is an optional Let's Encrypt certificate contact, not an SMTP login.
   You can leave email empty. For a mail server without login, leave both
   smtp.user and smtp.password empty as above. smtp.from is still required:
   it is the sender address, not a login. TLS is still required for delivery.

   JSON must have no comments or trailing commas. Unknown keys are errors.
   Backslashes in JSON strings need escaping: "C:\\Services\\CookieKill\\data".
   Relative paths are based on the executable directory, even when Windows
   starts the service with C:\Windows\System32 as its working directory.
   Restart the service after changing the file. Environment variables and
   executable parameters are not needed or used for production configuration.

3. Point public DNS for domain at this machine. Make TCP ports 80 and 443
   reachable through the host firewall, router/NAT, and any cloud firewall.
   Other software must not already be listening on these ports. HTTP port 80
   handles certificate validation and redirects visitors to HTTPS on port 443.
   Production startup accepts the Let's Encrypt subscriber agreement.
   The host also needs outbound DNS, HTTPS, and access to the SMTP provider.
   SMTP port 465 uses implicit TLS; other ports require STARTTLS. smtp.from must
   be an address the provider permits. Real mail delivery/certificate issuance
   must be checked using your domain and provider.

4. Open PowerShell AS ADMINISTRATOR in the extracted directory, then run:

   powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\Install-Service.ps1

   To explicitly add the Windows inbound firewall rule for TCP 80/443:

   powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\Install-Service.ps1 -OpenFirewall

   The optional -InstallDir argument selects an already extracted directory.
   The optional -NoStart argument registers/configures the service without
   starting it. Scripts do not copy or replace config.json or saved progress.

The service name and display name are CookieKill. It starts automatically at
boot, runs as NT AUTHORITY\LocalService, and restarts after failures (5 seconds,
15 seconds, then 60 seconds; failure count resets after one successful day).
Its registered executable path contains no command-line parameters. A normal
service stop saves progress before exiting and does not trigger recovery.

PERMISSIONS AND CONFIGURATION

The installer restricts the entire deployment directory to Administrators,
SYSTEM, and the CookieKill service SID. The service can read its configuration
and executable but can modify only the configured data and log directories.
Other ordinary users cannot read the stored SMTP password. Edit files from an
elevated editor after installation. Do not put unrelated files in this folder;
the installer resets its child-file permissions to the package policy.

For safe permission changes, the installer requires dataDir and logFile to be
inside the package directory, with data and logs in dedicated subdirectories.
It rejects symbolic links/junctions. Advanced manual service deployments may use
external absolute paths if permissions for NT SERVICE\CookieKill are configured
separately. Keep configuration and executable outside writable data/log folders.

Configuration fields:
  dev           false for public HTTPS; true for local-only testing
  addr          loopback HTTP address in dev mode; default 127.0.0.1:8080
  domain        lowercase public hostname, required in production
  email         optional Let's Encrypt contact address; empty is allowed
  dataDir       saved profiles and certificate cache; default data
  logFile       append-only server log; default logs/cookiekill.log
  smtp.host     mail server hostname, required in production
  smtp.port     default 587; 465 for implicit TLS, otherwise STARTTLS
  smtp.user     optional provider username; use together with password
  smtp.password optional provider password; use together with user
  smtp.from     email sender, required in production

To test on one computer without public DNS/mail, set dev to true, keep domain
empty, and use a literal loopback addr. Development login codes are displayed in
the browser. Never expose development mode publicly. Restore production values
before public deployment.

OPERATIONS (ELEVATED POWERSHELL)

  Get-Service CookieKill
  Stop-Service CookieKill
  Start-Service CookieKill
  Restart-Service CookieKill
  Get-Content .\logs\cookiekill.log -Tail 50

Use https://your-domain/healthz to check HTTP health. Check the configured logFile
for runtime errors. Startup/configuration failures are also recorded in Event
Viewer > Windows Logs > Application with source CookieKill (event ID 1).
The installer creates that event source. Service Control Manager errors are in
Windows Logs > System. The log is append-only; arrange retention/archiving, or
stop the service before moving the log aside, then start it to create a new one.

Saved profiles are in data\players.json; certificates are in data\certificates.
Back up config.json and the entire data directory securely. Stop the service
before a consistent backup. Only run one instance against a given data directory.

UPDATING AN EXISTING DEPLOYMENT

1. Extract the new ZIP into a separate temporary directory.
2. Stop-Service CookieKill, then back up the existing config.json and data.
3. Copy cookiekill.exe, Install-Service.ps1, Uninstall-Service.ps1, README.txt,
   and THIRD-PARTY-NOTICES.txt from the new package over the installed versions.
   Keep the installed config.json, data, and logs. Do not extract the complete
   ZIP over an existing installation: its blank config.json is a template.
4. Review the new configuration fields in the template and merge any needed
   settings into the existing config.json.
5. Run the installed Install-Service.ps1 again as administrator. It reapplies
   service settings/permissions, preserves configuration and progress, and starts
   the service. The installer refuses to replace a service registered elsewhere.

Restarting requires users to sign in again. Saved progress remains available.
Do not move the deployed directory while the service is registered to it.

REMOVAL

  powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\Uninstall-Service.ps1

This stops/removes the service and removes its optional firewall rule. It leaves
the executable, config.json, data, logs, permissions, and Event Log source intact.
Delete files yourself only after backing up anything you need. Close Services
management windows if Windows reports a service is still marked for deletion.

BUILDING THIS PACKAGE FROM SOURCE

  powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1

For ARM64 add -Architecture arm64. The output is dist\CookieKill-windows-amd64.zip
(or arm64) with a companion SHA-256 file. The build embeds the web files and
collects the full Go/module/Three.js license notices. Building needs Go 1.26+
and access to the module dependencies. Only actual service installation/removal
needs administrator privileges.

Windows service reference:
https://learn.microsoft.com/en-us/windows/win32/services/configuring-a-service-using-sc
