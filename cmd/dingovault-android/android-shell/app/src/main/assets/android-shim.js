(function () {
  if (window.__DINGO_ANDROID_SHIM__) return;
  window.__DINGO_ANDROID_SHIM__ = true;

  window.__dingoResolvers = {};
  window.__dvEvs = {};

  window.__dingoResolve = function (id, jsonStr) {
    var entry = window.__dingoResolvers[id];
    if (!entry) return;
    delete window.__dingoResolvers[id];
    try {
      var o = typeof jsonStr === 'string' ? JSON.parse(jsonStr) : jsonStr;
      if (o.ok) {
        if (Object.prototype.hasOwnProperty.call(o, 'data')) entry.res(o.data);
        else entry.res(undefined);
      } else {
        entry.rej(new Error(o.error || 'go'));
      }
    } catch (e) {
      entry.rej(e);
    }
  };

  function call(method, args) {
    return new Promise(function (res, rej) {
      var id = 'p' + Date.now() + Math.random().toString(36).slice(2);
      window.__dingoResolvers[id] = { res: res, rej: rej };
      try {
        AndroidBridge.call(method, JSON.stringify(args || []), id);
      } catch (e) {
        delete window.__dingoResolvers[id];
        rej(e);
      }
    });
  }

  window.go = { bridge: { App: {} } };
  var methods = [
    'AIChat',
    'ApplySlashOp',
    'CycleBlockTodo',
    'EnsurePage',
    'ExportPageHTML',
    'GetAISettings',
    'GetAppVersion',
    'GetBacklinks',
    'GetLocale',
    'GetPage',
    'GetSemanticGraphEdges',
    'GetSemanticRelatedForPage',
    'GetSyncSettings',
    'GetTheme',
    'GetWikiGraph',
    'IndentBlock',
    'InsertBlockAfter',
    'ListLANSyncPeers',
    'ListPagesByProperty',
    'ListVaultPages',
    'NotesRoot',
    'OutdentBlock',
    'PairLANSyncWith',
    'QueryBlocks',
    'ReorderBlockBefore',
    'ResolveWikilink',
    'SearchBlocks',
    'SetAISettings',
    'SetLocale',
    'SetSyncSettings',
    'SetTheme',
    'StartAIInlineStream',
    'StartLANSyncAdvertise',
    'StopLANSyncAdvertise',
    'Startup',
    'SuggestTagsForBlock',
    'SyncVaultWebDAV',
    'SyncVaultS3',
    'UpdateBlock'
  ];
  methods.forEach(function (m) {
    window.go.bridge.App[m] = function () {
      return call(m, Array.prototype.slice.call(arguments));
    };
  });

  window.runtime = {
    EventsOnMultiple: function (eventName, callback) {
      if (!window.__dvEvs[eventName]) window.__dvEvs[eventName] = [];
      window.__dvEvs[eventName].push(callback);
    },
    EventsOn: function (eventName, callback) {
      window.runtime.EventsOnMultiple(eventName, callback, -1);
    },
    EventsOff: function () {},
    EventsOffAll: function () {},
    EventsOnce: function (n, c) {
      window.runtime.EventsOnMultiple(n, c, 1);
    },
    LogPrint: function () {},
    LogTrace: function () {},
    LogDebug: function () {},
    LogInfo: function () {},
    LogWarning: function () {},
    LogError: function () {},
    LogFatal: function () {}
  };

  // Long init: show a hint if the Go bridge has not completed first paint work yet.
  window.__dingoFrontendReadyHooked = false;
  window.__dingoMarkFrontendReady = function () {
    window.__dingoFrontendReadyHooked = true;
    if (window.__dingoInitHintTimer) clearTimeout(window.__dingoInitHintTimer);
    var h = document.getElementById('dingo-init-hint');
    if (h) h.remove();
    try {
      if (typeof AndroidBridge !== 'undefined' && AndroidBridge.notifyFrontendReady) {
        AndroidBridge.notifyFrontendReady();
      }
    } catch (e) {}
  };
  function dingoShowInitHint() {
    if (window.__dingoFrontendReadyHooked || window.__dingoInitHintShown) return;
    window.__dingoInitHintShown = true;
    if (!document.body) return;
    var el = document.createElement('div');
    el.id = 'dingo-init-hint';
    el.setAttribute(
      'style',
      'position:fixed;left:12px;right:12px;bottom:max(24px,env(safe-area-inset-bottom));z-index:99999;' +
        'text-align:center;font:15px/1.45 system-ui,-apple-system,sans-serif;color:rgba(232,232,236,0.92);' +
        'background:rgba(18,18,22,0.92);border:1px solid rgba(255,255,255,0.12);border-radius:12px;padding:12px 14px;pointer-events:none;'
    );
    el.textContent = 'Initializing database…';
    document.body.appendChild(el);
  }
  window.__dingoInitHintTimer = setTimeout(function () {
    if (!window.__dingoFrontendReadyHooked) dingoShowInitHint();
  }, 8000);
})();
