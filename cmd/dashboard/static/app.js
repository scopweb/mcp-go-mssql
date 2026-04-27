// MCP-Go-MSSQL Configuration Builder
// Vanilla JS - No frameworks, no dependencies

(function() {
    'use strict';

    // DOM Elements
    var serverInput = document.getElementById('server');
    var databaseInput = document.getElementById('database');
    var portInput = document.getElementById('port');
    var authSelect = document.getElementById('auth');
    var userInput = document.getElementById('user');
    var passwordInput = document.getElementById('password');
    var sqlAuthFields = document.getElementById('sql-auth-fields');

    var developerModeCheckbox = document.getElementById('developer-mode');
    var encryptCheckbox = document.getElementById('encrypt');
    var encryptWarning = document.getElementById('encrypt-warning');

    var readOnlyCheckbox = document.getElementById('read-only');
    var whitelistInput = document.getElementById('whitelist');
    var allowedDatabasesInput = document.getElementById('allowed-databases');
    var confirmDestructiveCheckbox = document.getElementById('confirm-destructive');

    var autopilotCheckbox = document.getElementById('autopilot');
    var autopilotInfo = document.getElementById('autopilot-info');

    var configNameInput = document.getElementById('config-name');
    var outputTextarea = document.getElementById('output');
    var copyBtn = document.getElementById('copy-btn');
    var downloadBtn = document.getElementById('download-btn');

    // Show/hide SQL auth fields based on auth type
    authSelect.addEventListener('change', function() {
        var isSqlAuth = authSelect.value === 'sql';
        sqlAuthFields.style.opacity = isSqlAuth ? '1' : '0.4';
        userInput.disabled = !isSqlAuth;
        passwordInput.disabled = !isSqlAuth;
        if (!isSqlAuth) {
            userInput.value = '';
            passwordInput.value = '';
        }
    });

    // Encrypt warning for production mode
    developerModeCheckbox.addEventListener('change', updateEncryptWarning);
    encryptCheckbox.addEventListener('change', updateEncryptWarning);

    function updateEncryptWarning() {
        var devMode = developerModeCheckbox.checked;
        var encrypt = encryptCheckbox.checked;

        if (!devMode && !encrypt) {
            encryptWarning.classList.remove('hidden');
        } else {
            encryptWarning.classList.add('hidden');
        }

        // In production mode, force encrypt to true
        if (!devMode) {
            encryptCheckbox.checked = true;
            encryptCheckbox.disabled = true;
        } else {
            encryptCheckbox.disabled = false;
        }
    }

    // Autopilot info toggle
    autopilotCheckbox.addEventListener('change', function() {
        if (autopilotCheckbox.checked) {
            autopilotInfo.classList.remove('hidden');
            confirmDestructiveCheckbox.checked = false;
            confirmDestructiveCheckbox.disabled = true;
        } else {
            autopilotInfo.classList.add('hidden');
            confirmDestructiveCheckbox.disabled = false;
        }
    });

    // Read-only mode affects whitelist requirement
    readOnlyCheckbox.addEventListener('change', function() {
        if (!readOnlyCheckbox.checked) {
            whitelistInput.placeholder = 'Opcional - sin el, todas las modificaciones estan bloqueadas';
        } else {
            whitelistInput.placeholder = 'temp_ai, v_temp_ia, mi_vista';
        }
    });

    // Generate configuration
    function generateConfig() {
        var name = configNameInput.value || 'mcp-go-mssql';
        var env = {};

        // Required fields
        if (serverInput.value.trim()) {
            env.MSSQL_SERVER = serverInput.value.trim();
        }
        if (databaseInput.value.trim()) {
            env.MSSQL_DATABASE = databaseInput.value.trim();
        }
        if (portInput.value.trim() && portInput.value !== '1433') {
            env.MSSQL_PORT = portInput.value.trim();
        }

        // Auth type
        var authType = authSelect.value;
        if (authType === 'integrated') {
            env.MSSQL_AUTH = 'integrated';
        } else if (authType === 'azure') {
            env.MSSQL_AUTH = 'azure';
        } else {
            if (userInput.value.trim()) {
                env.MSSQL_USER = userInput.value.trim();
            }
            if (passwordInput.value) {
                env.MSSQL_PASSWORD = passwordInput.value;
            }
        }

        // Developer mode
        if (developerModeCheckbox.checked) {
            env.DEVELOPER_MODE = 'true';
        } else {
            env.DEVELOPER_MODE = 'false';
        }

        // Encrypt (only if dev mode)
        if (developerModeCheckbox.checked && !encryptCheckbox.checked) {
            env.MSSQL_ENCRYPT = 'false';
        }

        // Read only mode
        if (readOnlyCheckbox.checked) {
            env.MSSQL_READ_ONLY = 'true';
            if (whitelistInput.value.trim()) {
                env.MSSQL_WHITELIST_TABLES = whitelistInput.value.trim();
            }
        }

        // Allowed databases
        if (allowedDatabasesInput.value.trim()) {
            env.MSSQL_ALLOWED_DATABASES = allowedDatabasesInput.value.trim();
        }

        // Confirm destructive
        if (!confirmDestructiveCheckbox.checked && !autopilotCheckbox.checked) {
            env.MSSQL_CONFIRM_DESTRUCTIVE = 'false';
        }

        // Autopilot
        if (autopilotCheckbox.checked) {
            env.MSSQL_AUTOPILOT = 'true';
        }

        // Build config object
        var config = {
            name: name,
            command: 'mcp-go-mssql.exe',
            env: env
        };

        outputTextarea.value = JSON.stringify(config, null, 2);
    }

    // Copy to clipboard using modern API with fallback
    copyBtn.addEventListener('click', function() {
        var text = outputTextarea.value;

        // Try modern clipboard API first
        if (navigator.clipboard && navigator.clipboard.writeText) {
            navigator.clipboard.writeText(text).then(function() {
                showCopyFeedback(copyBtn, 'Copiado!');
            }).catch(function() {
                fallbackCopy(copyBtn);
            });
        } else {
            fallbackCopy(text, copyBtn);
        }
    });

    // Fallback copy function using textarea selection
    function fallbackCopy(btn) {
        outputTextarea.select();
        outputTextarea.setSelectionRange(0, 99999);
        var success = false;
        try {
            success = document.execCommand('copy');
        } catch (err) {
            success = false;
        }
        showCopyFeedback(btn, success ? 'Copiado!' : 'Error');
    }

    // Show feedback message
    function showCopyFeedback(btn, message) {
        var originalText = btn.textContent;
        btn.textContent = message;
        setTimeout(function() {
            btn.textContent = originalText;
        }, 2000);
    }

    // Download using data URI (avoids blob URL heuristics)
    downloadBtn.addEventListener('click', function() {
        var config = outputTextarea.value;
        var filename = (configNameInput.value || 'mcp-go-mssql') + '.mcp.json';

        // Encode as data URI (simpler, less likely to trigger AV)
        var dataUri = 'data:application/json;charset=utf-8,' + encodeURIComponent(config);

        // Create temporary link and trigger download
        var link = document.createElement('link');
        link.rel = 'prefetch';
        link.href = dataUri;
        link.setAttribute('download', filename);

        // Use click simulation via mouse event
        var event = new MouseEvent('click', {
            view: window,
            bubbles: true,
            cancelable: true
        });
        link.dispatchEvent(event);
    });

    // Add listeners to all inputs
    var allInputs = [
        serverInput, databaseInput, portInput, authSelect, userInput, passwordInput,
        developerModeCheckbox, encryptCheckbox, readOnlyCheckbox, whitelistInput,
        allowedDatabasesInput, confirmDestructiveCheckbox, autopilotCheckbox, configNameInput
    ];

    allInputs.forEach(function(input) {
        if (input) {
            input.addEventListener('input', generateConfig);
            input.addEventListener('change', generateConfig);
        }
    });

    // Initial config generation
    generateConfig();

})();