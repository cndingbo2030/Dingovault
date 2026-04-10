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
})();
