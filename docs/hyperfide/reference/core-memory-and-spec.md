# Source reference: C core: memory and spec

This part of the [source reference](../SOURCE-REFERENCE.md) lists 613 selected declarations in 10 files of the [Hyperfide workspace](../../../tools/hyperfide). Paths are relative to that workspace. Line numbers and quoted declarations refer to the linked source files. The declaration column shows the first source line, shortened to 200 characters where necessary. The tables include implementation helpers as well as public declarations.

## core/native/src/agent.c

[Open source](../../../tools/hyperfide/core/native/src/agent.c)

| Indexed name            | Kind     | Line | Source declaration                                                                                      |
| ----------------------- | -------- | ---: | ------------------------------------------------------------------------------------------------------- |
| `owned_write_at`        | function |   16 | <code>static int owned_write_at(const char &#42;,const char &#42;,size_t,const char &#42;,bool);</code> |
| `hyperfide_owned_write` | function |   17 | <code>int hyperfide_owned_write(const char &#42;filename,const char &#42;data,size_t length,</code>     |
| `owned_write_at`        | function |   50 | <code>static int owned_write_at(const char &#42;filename,const char &#42;data,size_t length,</code>     |
| `DirAgg`                | typedef  |  107 | <code>typedef struct { char name&#91;128&#93;; int files; long lines;</code>                            |
| `dir_lang`              | function |  111 | <code>static void dir_lang(DirAgg &#42;d, const char &#42;lang) {</code>                                |
| `TOOLING`               | variable |  131 | <code>static const struct { const char &#42;file, &#42;what; } TOOLING&#91;&#93; = {</code>             |
| `cmd_agentmd`           | function |  151 | <code>int cmd_agentmd(Cg &#42;cg, bool write_files) {</code>                                            |

## core/native/src/codemap.c

[Open source](../../../tools/hyperfide/core/native/src/codemap.c)

| Indexed name            | Kind     | Line | Source declaration                                                                                                          |
| ----------------------- | -------- | ---: | --------------------------------------------------------------------------------------------------------------------------- |
| `CM_MARKER`             | macro    |   19 | <code>#define CM_MARKER "&lt;!-- hyperfide-owned: codemap v1"</code>                                                        |
| `CM_LEGACY_MARKER`      | macro    |   20 | <code>#define CM_LEGACY_MARKER "&lt;!-- codify-owned: codemap v1"</code>                                                    |
| `hyperfide_owned_write` | function |   21 | <code>extern int hyperfide_owned_write(const char &#42;,const char &#42;,size_t,const char &#42;,bool);</code>              |
| `CM_BUDGET`             | macro    |   22 | <code>#define CM_BUDGET 8000</code>                                                                                         |
| `CM_MIN_BUDGET`         | macro    |   23 | <code>#define CM_MIN_BUDGET 300</code>                                                                                      |
| `CAP_MOD_SYMS`          | macro    |   26 | <code>#define CAP_MOD_SYMS 12</code>                                                                                        |
| `CAP_MOD_FILES`         | macro    |   27 | <code>#define CAP_MOD_FILES 40</code>                                                                                       |
| `CAP_MODULES`           | macro    |   28 | <code>#define CAP_MODULES 24</code>                                                                                         |
| `CAP_TEST_FILES`        | macro    |   29 | <code>#define CAP_TEST_FILES 40</code>                                                                                      |
| `CAP_ROUTES`            | macro    |   30 | <code>#define CAP_ROUTES 25</code>                                                                                          |
| `CAP_DEPS`              | macro    |   31 | <code>#define CAP_DEPS 30</code>                                                                                            |
| `CAP_DOCS`              | macro    |   32 | <code>#define CAP_DOCS 24</code>                                                                                            |
| `CAP_SUBDIRS`           | macro    |   33 | <code>#define CAP_SUBDIRS 8</code>                                                                                          |
| `CAP_COMMANDS`          | macro    |   34 | <code>#define CAP_COMMANDS 96</code>                                                                                        |
| `codemap_default_path`  | function |   38 | <code>void codemap_default_path(const Cg &#42;cg, char &#42;out, size_t cap) {</code>                                       |
| `cm_spec_dir`           | function |   43 | <code>static const char &#42;cm_spec_dir(const Cg &#42;cg) {</code>                                                         |
| `Cut`                   | typedef  |   52 | <code>typedef struct { int tier; long rank; int seq; bool drop; } Cut;</code>                                               |
| `CmFile`                | typedef  |   54 | <code>typedef struct {</code>                                                                                               |
| `CmLang`                | typedef  |   61 | <code>typedef struct { char &#42;name; long files, lines; Cut cut; } CmLang;</code>                                         |
| `CmBuild`               | typedef  |   62 | <code>typedef struct { char &#42;tool, &#42;manifest, &#42;dir, &#42;cmds; Cut cut; } CmBuild;</code>                       |
| `CmDir`                 | typedef  |   64 | <code>typedef struct {</code>                                                                                               |
| `CmMain`                | typedef  |   72 | <code>typedef struct { char &#42;path, &#42;sig; char name&#91;128&#93;; int line; Cut cut; } CmMain;</code>                |
| `CmPkg`                 | typedef  |   73 | <code>typedef struct { char &#42;manifest, &#42;what; Cut cut; } CmPkg;</code>                                              |
| `CmRoute`               | typedef  |   74 | <code>typedef struct {</code>                                                                                               |
| `CmModFile`             | typedef  |   80 | <code>typedef struct { CmFile &#42;f; Cut cut; } CmModFile;</code>                                                          |
| `CmSym`                 | typedef  |   81 | <code>typedef struct {</code>                                                                                               |
| `CmMod`                 | typedef  |   87 | <code>typedef struct {</code>                                                                                               |
| `CmDep`                 | typedef  |   96 | <code>typedef struct { char &#42;from, &#42;to; long imports, calls; Cut cut; } CmDep;</code>                               |
| `CmTestFile`            | typedef  |   98 | <code>typedef struct { CmFile &#42;f; Cut cut; } CmTestFile;</code>                                                         |
| `CmTestDir`             | typedef  |   99 | <code>typedef struct {</code>                                                                                               |
| `CmFixture`             | typedef  |  105 | <code>typedef struct { char &#42;path, &#42;subs; long files; Cut cut; } CmFixture;</code>                                  |
| `CmPtr`                 | typedef  |  107 | <code>typedef struct { char &#42;path, &#42;what; Cut cut; } CmPtr;</code>                                                  |
| `Map`                   | typedef  |  109 | <code>typedef struct {</code>                                                                                               |
| `VPUSH`                 | macro    |  141 | <code>#define VPUSH(v, n, c) \</code>                                                                                       |
| `cut_init`              | function |  148 | <code>static void cut_init(Map &#42;m, Cut &#42;c, int tier, long rank) {</code>                                            |
| `cut_add`               | function |  154 | <code>static void cut_add(Map &#42;m, Cut &#42;c) {</code>                                                                  |
| `num`                   | function |  164 | <code>static const char &#42;num(long n, char &#42;buf) { /&#42; 39334 -&gt; "39,334" &#42;/</code>                         |
| `starts_ci`             | function |  177 | <code>static bool starts_ci(const char &#42;s, const char &#42;pre) {</code>                                                |
| `strip_leader`          | function |  182 | <code>static void strip_leader(const char &#42;&#42;p, int &#42;ll) {</code>                                                |
| `has_alnum`             | function |  204 | <code>static bool has_alnum(const char &#42;p, int ll) {</code>                                                             |
| `boilerplate`           | function |  210 | <code>static bool boilerplate(const char &#42;p, int ll) {</code>                                                           |
| `sentence`              | function |  222 | <code>static char &#42;sentence(const char &#42;s, int max) {</code>                                                        |
| `purpose_of`            | function |  250 | <code>static char &#42;purpose_of(const char &#42;body, int max) {</code>                                                   |
| `md_title`              | function |  279 | <code>static char &#42;md_title(const char &#42;body) {</code>                                                              |
| `md_plain`              | function |  299 | <code>static void md_plain(StrBuf &#42;b, const char &#42;p, int ll) {</code>                                               |
| `readme_prose`          | function |  311 | <code>static char &#42;readme_prose(const char &#42;body, int max) {</code>                                                 |
| `clean_sig`             | function |  340 | <code>static char &#42;clean_sig(const char &#42;sig) {</code>                                                              |
| `cm_skipped`            | function |  364 | <code>static bool cm_skipped(const Map &#42;m, const char &#42;path) {</code>                                               |
| `cm_fixture`            | function |  371 | <code>static bool cm_fixture(const char &#42;path) {</code>                                                                 |
| `cm_test`               | function |  381 | <code>static bool cm_test(const char &#42;path) {</code>                                                                    |
| `top_dir`               | function |  386 | <code>static void top_dir(const char &#42;path, char &#42;out, size_t cap) {</code>                                         |
| `mod_dir`               | function |  392 | <code>static void mod_dir(const char &#42;path, char &#42;out, size_t cap) {</code>                                         |
| `fn_skip`               | function |  401 | <code>static void fn_skip(sqlite3_context &#42;c, int n, sqlite3_value &#42;&#42;v) {</code>                                |
| `fn_test`               | function |  406 | <code>static void fn_test(sqlite3_context &#42;c, int n, sqlite3_value &#42;&#42;v) {</code>                                |
| `fn_top`                | function |  411 | <code>static void fn_top(sqlite3_context &#42;c, int n, sqlite3_value &#42;&#42;v) {</code>                                 |
| `fn_mod`                | function |  418 | <code>static void fn_mod(sqlite3_context &#42;c, int n, sqlite3_value &#42;&#42;v) {</code>                                 |
| `cm_functions`          | function |  426 | <code>static void cm_functions(Map &#42;m, bool on) {</code>                                                                |
| `cm_prep`               | function |  440 | <code>static sqlite3_stmt &#42;cm_prep(Map &#42;m, const char &#42;alias, const char &#42;head,</code>                      |
| `col`                   | function |  451 | <code>static const char &#42;col(sqlite3_stmt &#42;st, int i) {</code>                                                      |
| `byid_cmp`              | function |  456 | <code>static int byid_cmp(const void &#42;a, const void &#42;b) {</code>                                                    |
| `file_by_id`            | function |  461 | <code>static CmFile &#42;file_by_id(Map &#42;m, long id) {</code>                                                           |
| `file_by_path`          | function |  471 | <code>static CmFile &#42;file_by_path(Map &#42;m, const char &#42;path) {</code>                                            |
| `dir_range`             | function |  485 | <code>static void dir_range(const Map &#42;m, const char &#42;dir, int &#42;lo, int &#42;hi) {</code>                       |
| `tree_read`             | function |  501 | <code>static char &#42;tree_read(Map &#42;m, const char &#42;rel) {</code>                                                  |
| `load_files`            | function |  509 | <code>static void load_files(Map &#42;m) {</code>                                                                           |
| `load_name`             | function |  566 | <code>static void load_name(Map &#42;m) {</code>                                                                            |
| `lang_cmp`              | function |  626 | <code>static int lang_cmp(const void &#42;a, const void &#42;b) {</code>                                                    |
| `load_langs`            | function |  633 | <code>static void load_langs(Map &#42;m) {</code>                                                                           |
| `dir_of`                | function |  656 | <code>static void dir_of(const char &#42;path, char &#42;out, size_t cap) {</code>                                          |
| `add_build`             | function |  661 | <code>static void add_build(Map &#42;m, const char &#42;tool, const char &#42;manifest,</code>                              |
| `make_targets`          | function |  677 | <code>static void make_targets(const char &#42;body, StrBuf &#42;b) {</code>                                                |
| `load_builds`           | function |  711 | <code>static void load_builds(Map &#42;m) {</code>                                                                          |
| `dir_purpose`           | function |  780 | <code>static char &#42;dir_purpose(Map &#42;m, const char &#42;dir) {</code>                                                |
| `dir_get`               | function |  812 | <code>static CmDir &#42;dir_get(Map &#42;m, const char &#42;path, int depth) {</code>                                       |
| `dir_cmp`               | function |  821 | <code>static int dir_cmp(const void &#42;a, const void &#42;b) {</code>                                                     |
| `load_layout`           | function |  825 | <code>static void load_layout(Map &#42;m) {</code>                                                                          |
| `LN`                    | typedef  |  827 | <code>typedef struct { char lang&#91;32&#93;; long n; } LN;</code>                                                          |
| `dir_listed`            | function |  921 | <code>static bool dir_listed(const Map &#42;m, const CmDir &#42;d) {</code>                                                 |
| `word_ok`               | function |  934 | <code>static bool word_ok(const char &#42;s, int n) {</code>                                                                |
| `cmd_push`              | function |  943 | <code>static void cmd_push(StrBuf &#42;b, int &#42;n, char seen&#91;&#93;&#91;48&#93;, const char &#42;s, int len) {</code> |
| `scan_commands`         | function |  956 | <code>static int scan_commands(const char &#42;body, StrBuf &#42;b) {</code>                                                |
| `load_entries`          | function | 1002 | <code>static void load_entries(Map &#42;m) {</code>                                                                         |
| `modfile_cmp`           | function | 1099 | <code>static int modfile_cmp(const void &#42;a, const void &#42;b) {</code>                                                 |
| `mod_cmp`               | function | 1106 | <code>static int mod_cmp(const void &#42;a, const void &#42;b) {</code>                                                     |
| `mod_lookup`            | function | 1113 | <code>static CmMod &#42;mod_lookup(Map &#42;m, const char &#42;path) {</code>                                               |
| `load_modules`          | function | 1119 | <code>static void load_modules(Map &#42;m) {</code>                                                                         |
| `LN`                    | typedef  | 1141 | <code>typedef struct { char lang&#91;32&#93;; long n; } LN;</code>                                                          |
| `dep_get`               | function | 1237 | <code>static CmDep &#42;dep_get(Map &#42;m, const char &#42;a, const char &#42;b) {</code>                                  |
| `dep_cmp`               | function | 1247 | <code>static int dep_cmp(const void &#42;a, const void &#42;b) {</code>                                                     |
| `load_deps`             | function | 1255 | <code>static void load_deps(Map &#42;m) {</code>                                                                            |
| `script_purpose`        | function | 1285 | <code>static char &#42;script_purpose(Map &#42;m, const char &#42;path) {</code>                                            |
| `tdir_lookup`           | function | 1307 | <code>static CmTestDir &#42;tdir_lookup(Map &#42;m, const char &#42;path) {</code>                                          |
| `fixture_root`          | function | 1314 | <code>static void fixture_root(const char &#42;path, char &#42;out, size_t cap) {</code>                                    |
| `tfile_cmp`             | function | 1327 | <code>static int tfile_cmp(const void &#42;a, const void &#42;b) {</code>                                                   |
| `tdir_cmp`              | function | 1333 | <code>static int tdir_cmp(const void &#42;a, const void &#42;b) {</code>                                                    |
| `load_tests`            | function | 1337 | <code>static void load_tests(Map &#42;m) {</code>                                                                           |
| `add_ptr`               | function | 1449 | <code>static void add_ptr(Map &#42;m, const char &#42;path, const char &#42;what) {</code>                                  |
| `load_pointers`         | function | 1456 | <code>static void load_pointers(Map &#42;m) {</code>                                                                        |
| `kept`                  | function | 1557 | <code>static bool kept(const Cut &#42;c) { return !c-&gt;drop; }</code>                                                     |
| `md_section_omitted`    | function | 1559 | <code>static void md_section_omitted(StrBuf &#42;b, const char &#42;what) {</code>                                          |
| `omit_add`              | function | 1563 | <code>static void omit_add(StrBuf &#42;o, long n, const char &#42;one, const char &#42;many) {</code>                       |
| `render_md`             | function | 1570 | <code>static void render_md(Map &#42;m, StrBuf &#42;b) {</code>                                                             |
| `js_str_or_null`        | function | 1862 | <code>static void js_str_or_null(StrBuf &#42;b, const char &#42;s) {</code>                                                 |
| `render_json`           | function | 1866 | <code>static void render_json(Map &#42;m, StrBuf &#42;b, long md_len) {</code>                                              |
| `cut_cmp`               | function | 2117 | <code>static int cut_cmp(const void &#42;a, const void &#42;b) {</code>                                                     |
| `md_len_dropping`       | function | 2124 | <code>static size_t md_len_dropping(Map &#42;m, int k) {</code>                                                             |
| `collect_cuts`          | function | 2133 | <code>static void collect_cuts(Map &#42;m) {</code>                                                                         |
| `fit`                   | function | 2164 | <code>static int fit(Map &#42;m, size_t cap) {</code>                                                                       |
| `map_free`              | function | 2177 | <code>static void map_free(Map &#42;m) {</code>                                                                             |
| `codemap_render`        | function | 2230 | <code>int codemap_render(Cg &#42;cg, int budget, bool json, const char &#42;self_rel,</code>                                |
| `marker_budget`         | function | 2272 | <code>static int marker_budget(const char &#42;body) {</code>                                                               |
| `out_rel`               | function | 2284 | <code>static void out_rel(const Cg &#42;cg, const char &#42;abs, char &#42;rel, size_t cap) {</code>                        |
| `codemap_status`        | function | 2292 | <code>int codemap_status(Cg &#42;cg, char &#42;rel, size_t cap) {</code>                                                    |
| `cmd_codemap`           | function | 2308 | <code>int cmd_codemap(Cg &#42;cg, const CodemapOpts &#42;o) {</code>                                                        |

## core/native/src/config.c

[Open source](../../../tools/hyperfide/core/native/src/config.c)

| Indexed name                        | Kind     | Line | Source declaration                                                                                        |
| ----------------------------------- | -------- | ---: | --------------------------------------------------------------------------------------------------------- |
| `hyperfide_environment_aliases`     | function |   38 | <code>int hyperfide_environment_aliases(void) {</code>                                                    |
| `names_metadata`                    | function |  124 | <code>static char &#42;names_metadata(const char &#42;filename) {</code>                                  |
| `names_migration_selects_canonical` | function |  129 | <code>static bool names_migration_selects_canonical(const char &#42;root,const char &#42;legacy) {</code> |
| `config_selected_path`              | function |  156 | <code>static void config_selected_path(const char &#42;root,char &#42;out,size_t capacity) {</code>       |
| `CfgType`                           | typedef  |  168 | <code>typedef enum { CFG_BOOL, CFG_PATH, CFG_INT, CFG_STR, CFG_DUR,</code>                                |
| `CFG_KEYS`                          | variable |  177 | <code>} CFG_KEYS&#91;CFG_NKEYS&#93; = {</code>                                                            |
| `CfgIssue`                          | typedef  |  260 | <code>typedef struct {</code>                                                                             |
| `CfgIssues`                         | typedef  |  267 | <code>typedef struct { CfgIssue &#42;v; int n, cap; } CfgIssues;</code>                                   |
| `cfg_issue`                         | function |  269 | <code>static void cfg_issue(CfgIssues &#42;is, const char &#42;kind, const char &#42;section,</code>      |
| `cfg_issue`                         | function |  269 | <code>static void cfg_issue(CfgIssues &#42;is, const char &#42;kind, const char &#42;section,</code>      |
| `cfg_index`                         | function |  291 | <code>static int cfg_index(const char &#42;section, const char &#42;key) {</code>                         |
| `cfg_index_dotted`                  | function |  300 | <code>static int cfg_index_dotted(const char &#42;dotted) {</code>                                        |
| `cfg_known_section`                 | function |  308 | <code>static bool cfg_known_section(const char &#42;section) {</code>                                     |
| `cfg_parse_bool`                    | function |  315 | <code>static int cfg_parse_bool(const char &#42;v) {</code>                                               |
| `config_parse_workers`              | function |  323 | <code>int config_parse_workers(const char &#42;v) {</code>                                                |
| `config_parse_walk`                 | function |  337 | <code>int config_parse_walk(const char &#42;v) {</code>                                                   |
| `walk_name`                         | function |  346 | <code>static const char &#42;walk_name(int w) {</code>                                                    |
| `cfg_path_check`                    | function |  353 | <code>static const char &#42;cfg_path_check(const char &#42;v, char &#42;out, size_t cap) {</code>        |
| `cfg_slot`                          | function |  385 | <code>static char &#42;cfg_slot(CgConfig &#42;c, int i) {</code>                                          |
| `cfg_defaults`                      | function |  406 | <code>static void cfg_defaults(CgConfig &#42;c, const char &#42;root) {</code>                            |
| `cfg_read`                          | function |  427 | <code>static void cfg_read(const char &#42;root, CgConfig &#42;c, CfgIssues &#42;is) {</code>             |
| `CfgNode`                           | struct   |  560 | <code>typedef struct CfgNode { CgConfig c; struct CfgNode &#42;next; } CfgNode;</code>                    |
| `cfg_file_name`                     | function |  568 | <code>static const char &#42;cfg_file_name(const CgConfig &#42;c) {</code>                                |
| `config_file_name`                  | function |  572 | <code>const char &#42;config_file_name(const char &#42;root) {</code>                                     |
| `config_load`                       | function |  576 | <code>const CgConfig &#42;config_load(const char &#42;root) {</code>                                      |
| `config_auto_sync`                  | function |  606 | <code>bool config_auto_sync(const char &#42;root) { return config_load(root)-&gt;sync_auto; }</code>      |
| `config_spec_rel`                   | function |  607 | <code>const char &#42;config_spec_rel(const char &#42;root) {</code>                                      |
| `config_context_rel`                | function |  610 | <code>const char &#42;config_context_rel(const char &#42;root) {</code>                                   |
| `config_skills_rel`                 | function |  613 | <code>const char &#42;config_skills_rel(const char &#42;root) {</code>                                    |
| `config_codemap_rel`                | function |  616 | <code>const char &#42;config_codemap_rel(const char &#42;root) {</code>                                   |
| `config_index_workers`              | function |  619 | <code>int config_index_workers(const char &#42;root) {</code>                                             |
| `config_projection_base`            | function |  623 | <code>const char &#42;config_projection_base(const char &#42;root) {</code>                               |
| `config_projection_stale`           | function |  626 | <code>long config_projection_stale(const char &#42;root) {</code>                                         |
| `config_projection_index`           | function |  629 | <code>bool config_projection_index(const char &#42;root) {</code>                                         |
| `config_fleet_discover`             | function |  633 | <code>bool config_fleet_discover(const char &#42;root) {</code>                                           |
| `config_projection_steer`           | function |  636 | <code>bool config_projection_steer(const char &#42;root) { /&#42; codify-v15 3.2 &#42;/</code>            |
| `config_shell_advice`               | function |  639 | <code>bool config_shell_advice(const char &#42;root) { /&#42; codify-v15 1.4 &#42;/</code>                |
| `config_projection_auto`            | function |  643 | <code>bool config_projection_auto(const char &#42;root) {</code>                                          |
| `config_projection_min_interval`    | function |  646 | <code>long config_projection_min_interval(const char &#42;root) {</code>                                  |
| `config_projection_exclude`         | function |  649 | <code>int config_projection_exclude(const char &#42;root, char &#42;&#42;&#42;out) {</code>               |
| `config_index_walk`                 | function |  666 | <code>int config_index_walk(const char &#42;root) {</code>                                                |
| `config_spec_dir`                   | function |  670 | <code>bool config_spec_dir(const char &#42;root, char &#42;out, size_t cap) {</code>                      |
| `config_workflow_path`              | function |  674 | <code>bool config_workflow_path(const char &#42;root, char &#42;out, size_t cap) {</code>                 |
| `config_feature_path`               | function |  679 | <code>bool config_feature_path(const char &#42;root, const char &#42;feature, char &#42;out,</code>       |
| `config_context_dir`                | function |  685 | <code>bool config_context_dir(const char &#42;root, char &#42;out, size_t cap) {</code>                   |
| `config_context_path`               | function |  689 | <code>bool config_context_path(const char &#42;root, const char &#42;name, char &#42;out,</code>          |
| `config_skills_dir`                 | function |  695 | <code>bool config_skills_dir(const char &#42;root, char &#42;out, size_t cap) {</code>                    |
| `config_codemap_path`               | function |  699 | <code>bool config_codemap_path(const char &#42;root, char &#42;out, size_t cap) {</code>                  |
| `config_in_spec`                    | function |  703 | <code>bool config_in_spec(const char &#42;root, const char &#42;rel) {</code>                             |
| `config_find_spec_root`             | function |  709 | <code>int config_find_spec_root(const char &#42;start, char &#42;out, size_t cap) {</code>                |
| `cfg_issues_json`                   | function |  728 | <code>static void cfg_issues_json(const CgConfig &#42;c, const CfgIssues &#42;is,</code>                  |
| `config_check`                      | function |  751 | <code>int config_check(const char &#42;root, StrBuf &#42;text, StrBuf &#42;json) {</code>                 |
| `cfg_core_warning`                  | function |  767 | <code>static void cfg_core_warning(const char &#42;root, char &#42;out, size_t cap) {</code>              |
| `cfg_root`                          | function |  786 | <code>static void cfg_root(char &#42;out, size_t cap) {</code>                                            |
| `cfg_value`                         | function |  797 | <code>static const char &#42;cfg_value(const CgConfig &#42;c, int i) {</code>                             |
| `cfg_setting_json`                  | function |  818 | <code>static void cfg_setting_json(const CgConfig &#42;c, int i, StrBuf &#42;b) {</code>                  |
| `cfg_list`                          | function |  856 | <code>static int cfg_list(const char &#42;root, bool json) {</code>                                       |
| `cfg_init`                          | function |  893 | <code>static int cfg_init(const char &#42;root) {</code>                                                  |
| `cfg_get`                           | function |  910 | <code>static int cfg_get(const char &#42;root, const char &#42;dotted, bool json) {</code>                |
| `cfg_set`                           | function |  930 | <code>static int cfg_set(const char &#42;root, const char &#42;dotted, const char &#42;value) {</code>    |
| `cmd_config`                        | function | 1031 | <code>int cmd_config(int argc, char &#42;&#42;argv, bool json) {</code>                                   |

## core/native/src/docs.c

[Open source](../../../tools/hyperfide/core/native/src/docs.c)

| Indexed name               | Kind     | Line | Source declaration                                                                                     |
| -------------------------- | -------- | ---: | ------------------------------------------------------------------------------------------------------ |
| `DOCS_EVIDENCE_CAP`        | macro    |   13 | <code>#define DOCS_EVIDENCE_CAP 16000</code>                                                           |
| `DOCS_INVENTORY_CAP`       | macro    |   14 | <code>#define DOCS_INVENTORY_CAP 256</code>                                                            |
| `DocsProject`              | typedef  |   19 | <code>typedef struct {</code>                                                                          |
| `docs_str`                 | function |   36 | <code>static char &#42;docs_str(const Kvx &#42;k, const char &#42;sec, const char &#42;key,</code>     |
| `docs_free_list`           | function |   42 | <code>static void docs_free_list(char &#42;&#42;v, int n) {</code>                                     |
| `docs_project_close`       | function |   47 | <code>static void docs_project_close(DocsProject &#42;p) {</code>                                      |
| `docs_defaults`            | function |   56 | <code>static int docs_defaults(char &#42;&#42;&#42;out, const char &#42;&#42;items, int n) {</code>    |
| `docs_tasks_qualified`     | function |   63 | <code>static bool docs_tasks_qualified(const Kvx &#42;spec) {</code>                                   |
| `docs_project_open`        | function |   85 | <code>static int docs_project_open(Cg &#42;cg, DocsProject &#42;p) {</code>                            |
| `docs_suffix`              | function |  155 | <code>static bool docs_suffix(const char &#42;name) {</code>                                           |
| `docs_inventory_walk`      | function |  161 | <code>static void docs_inventory_walk(const char &#42;root, const char &#42;rel, int depth,</code>     |
| `docs_inventory`           | function |  198 | <code>static int docs_inventory(const DocsProject &#42;p, StrBuf &#42;text, StrBuf &#42;json) {</code> |
| `docs_targets_json`        | function |  211 | <code>static void docs_targets_json(const DocsProject &#42;p, StrBuf &#42;b) {</code>                  |
| `docs_target_match`        | function |  226 | <code>static bool docs_target_match(const DocsProject &#42;p, const char &#42;path) {</code>           |
| `docs_regular`             | function |  237 | <code>static bool docs_regular(const DocsProject &#42;p, const char &#42;rel) {</code>                 |
| `docs_string_in_file`      | function |  250 | <code>static bool docs_string_in_file(const DocsProject &#42;p, const char &#42;rel,</code>            |
| `docs_symbol_exists`       | function |  261 | <code>static bool docs_symbol_exists(Cg &#42;cg, const char &#42;name, const char &#42;path) {</code>  |
| `docs_route_exists`        | function |  272 | <code>static bool docs_route_exists(Cg &#42;cg, const char &#42;value, const char &#42;path) {</code>  |
| `docs_plan`                | function |  492 | <code>static int docs_plan(Cg &#42;cg, bool json) {</code>                                             |
| `DocsSpecCall`             | typedef  |  529 | <code>typedef struct { int argc; char &#42;&#42;argv; bool json; } DocsSpecCall;</code>                |
| `docs_call_spec`           | function |  530 | <code>static int docs_call_spec(void &#42;v) {</code>                                                  |
| `DocsCall`                 | typedef  |  543 | <code>typedef struct { Cg &#42;cg; int which; const char &#42;feature; } DocsCall;</code>              |
| `docs_call_evidence`       | function |  544 | <code>static int docs_call_evidence(void &#42;v) {</code>                                              |
| `docs_append_capture`      | function |  557 | <code>static void docs_append_capture(StrBuf &#42;packet, StrBuf &#42;ledger,</code>                   |
| `docs_packet`              | function |  586 | <code>static int docs_packet(Cg &#42;cg, bool json) {</code>                                           |
| `DocsCheck`                | typedef  |  723 | <code>typedef struct { int errors; int checks; StrBuf report; } DocsCheck;</code>                      |
| `docs_check_say`           | function |  725 | <code>static void docs_check_say(DocsCheck &#42;c, bool ok, const char &#42;fmt, ...) {</code>         |
| `docs_allowed_system_path` | function |  738 | <code>static bool docs_allowed_system_path(const DocsProject &#42;p, const char &#42;path) {</code>    |
| `docs_check_links`         | function |  748 | <code>static int docs_check_links(const DocsProject &#42;p, const char &#42;doc,</code>                |
| `docs_claim_evidence`      | function |  797 | <code>static bool docs_claim_evidence(const DocsProject &#42;p, const char &#42;evidence,</code>       |
| `docs_check_claims`        | function |  809 | <code>static int docs_check_claims(DocsProject &#42;p, DocsCheck &#42;c) {</code>                      |
| `docs_check`               | function |  901 | <code>static int docs_check(Cg &#42;cg, bool json) {</code>                                            |
| `docs_trace`               | function |  969 | <code>static int docs_trace(Cg &#42;cg, bool json) {</code>                                            |
| `docs_call_check`          | function | 1030 | <code>static int docs_call_check(void &#42;v) { return docs_check((Cg &#42;)v, false); }</code>        |
| `docs_call_finish`         | function | 1031 | <code>static int docs_call_finish(void &#42;v) {</code>                                                |
| `docs_close`               | function | 1036 | <code>static int docs_close(Cg &#42;cg, bool json) {</code>                                            |
| `cmd_docs`                 | function | 1078 | <code>int cmd_docs(Cg &#42;cg, int argc, char &#42;&#42;argv, bool json) {</code>                      |

## core/native/src/jev.c

[Open source](../../../tools/hyperfide/core/native/src/jev.c)

| Indexed name           | Kind     | Line | Source declaration                                                                                    |
| ---------------------- | -------- | ---: | ----------------------------------------------------------------------------------------------------- |
| `JEV_DEFAULT_MODEL`    | macro    |   35 | <code>#define JEV_DEFAULT_MODEL "typesafe/jev-1.13"</code>                                            |
| `JEV_DEFAULT_ENDPOINT` | macro    |   36 | <code>#define JEV_DEFAULT_ENDPOINT "https://openrouter.ai/api/alpha/decisions"</code>                 |
| `JEV_MAX_CHOICE`       | macro    |   37 | <code>#define JEV_MAX_CHOICE 255</code>                                                               |
| `JEV_EXCERPT`          | macro    |   38 | <code>#define JEV_EXCERPT 512</code>                                                                  |
| `JEV_MAX_ANSWERS`      | macro    |   39 | <code>#define JEV_MAX_ANSWERS 512</code>                                                              |
| `JevConfig`            | typedef  |   41 | <code>typedef struct {</code>                                                                         |
| `jev_config`           | function |   51 | <code>static void jev_config(JevConfig &#42;c) {</code>                                               |
| `dup_list`             | function |   73 | <code>static char &#42;&#42;dup_list(const char &#42;const &#42;v, int n) {</code>                    |
| `jev_question_noul`    | function |   80 | <code>int jev_question_noul(JevQuestion &#42;q, const char &#42;name,</code>                          |
| `jev_question_choice`  | function |   98 | <code>int jev_question_choice(JevQuestion &#42;q, const char &#42;name,</code>                        |
| `jev_question_score`   | function |  112 | <code>int jev_question_score(JevQuestion &#42;q, const char &#42;name,</code>                         |
| `jev_question_free`    | function |  125 | <code>void jev_question_free(JevQuestion &#42;q) {</code>                                             |
| `jev_type_name`        | function |  137 | <code>static const char &#42;jev_type_name(int t) {</code>                                            |
| `json_value_ok`        | function |  143 | <code>static bool json_value_ok(const char &#42;s) {</code>                                           |
| `cmp_str_idx`          | function |  175 | <code>static int cmp_str_idx(const void &#42;a, const void &#42;b, void &#42;arg) {</code>            |
| `sorted_order`         | function |  180 | <code>static void sorted_order(char &#42;&#42;v, int n, int &#42;idx) {</code>                        |
| `sb_question`          | function |  185 | <code>static void sb_question(StrBuf &#42;b, const JevQuestion &#42;q) {</code>                       |
| `jev_request_json`     | function |  214 | <code>void jev_request_json(const char &#42;model, const char &#42;state_json,</code>                 |
| `sb_cfgquote`          | function |  242 | <code>static void sb_cfgquote(StrBuf &#42;b, const char &#42;s) {</code>                              |
| `jev_tmp_dir`          | function |  255 | <code>static int jev_tmp_dir(const Cg &#42;cg, char &#42;out, size_t cap) {</code>                    |
| `write_private`        | function |  268 | <code>static int write_private(const char &#42;path, const char &#42;data) {</code>                   |
| `sleep_ms`             | function |  281 | <code>static void sleep_ms(long ms) {</code>                                                          |
| `excerpt_of`           | function |  287 | <code>static void excerpt_of(const char &#42;s, char &#42;out, size_t cap) {</code>                   |
| `curl_once`            | function |  299 | <code>static int curl_once(const char &#42;curl, const char &#42;cfg, char &#42;&#42;body,</code>     |
| `jnum`                 | function |  333 | <code>static double jnum(const char &#42;obj, const char &#42;key, bool &#42;ok) {</code>             |
| `jev_parse`            | function |  345 | <code>static int jev_parse(const char &#42;resp, JevResult &#42;out) {</code>                         |
| `jev_answer`           | function |  425 | <code>const JevAnswer &#42;jev_answer(const JevResult &#42;r, const char &#42;name) {</code>          |
| `jev_result_free`      | function |  431 | <code>void jev_result_free(JevResult &#42;r) {</code>                                                 |
| `jev_log_path`         | function |  443 | <code>static void jev_log_path(const Cg &#42;cg, char &#42;out, size_t cap) {</code>                  |
| `jev_log`              | function |  449 | <code>static void jev_log(const Cg &#42;cg, const JevConfig &#42;c, const JevResult &#42;r,</code>    |
| `count_questions`      | function |  492 | <code>static int count_questions(const char &#42;body) {</code>                                       |
| `jev_ask_raw_cfg`      | function |  502 | <code>static int jev_ask_raw_cfg(Cg &#42;cg, const JevConfig &#42;cp, const char &#42;body_in,</code> |
| `jev_ask_raw`          | function |  505 | <code>int jev_ask_raw(Cg &#42;cg, const char &#42;body_in, JevResult &#42;out) {</code>               |
| `jev_ask_raw_cfg`      | function |  511 | <code>static int jev_ask_raw_cfg(Cg &#42;cg, const JevConfig &#42;cp, const char &#42;body_in,</code> |
| `jev_ask`              | function |  622 | <code>int jev_ask(Cg &#42;cg, const char &#42;state_json, const JevQuestion &#42;qs, int nq,</code>   |
| `jev_ask_at`           | function |  642 | <code>int jev_ask_at(Cg &#42;cg, const char &#42;key, const char &#42;model,</code>                   |
| `print_probabilities`  | function |  671 | <code>static void print_probabilities(const char &#42;probs, StrBuf &#42;b) {</code>                  |
| `jev_result_json`      | function |  681 | <code>static void jev_result_json(const JevResult &#42;r, StrBuf &#42;b) {</code>                     |
| `jev_result_text`      | function |  711 | <code>static void jev_result_text(const JevResult &#42;r, StrBuf &#42;b) {</code>                     |
| `AskArgs`              | typedef  |  751 | <code>typedef struct {</code>                                                                         |
| `ask_push`             | function |  755 | <code>static JevQuestion &#42;ask_push(AskArgs &#42;a) {</code>                                       |
| `list_push`            | function |  765 | <code>static void list_push(char &#42;&#42;&#42;v, int &#42;n, const char &#42;s) {</code>            |
| `ask_usage`            | function |  770 | <code>static int ask_usage(void) {</code>                                                             |
| `jev_ask_cli`          | function |  779 | <code>static int jev_ask_cli(Cg &#42;cg, int argc, char &#42;&#42;argv, bool json) {</code>           |
| `curl_version`         | function |  925 | <code>static void curl_version(const char &#42;curl, char &#42;out, size_t cap) {</code>              |
| `key_hint`             | function |  947 | <code>static void key_hint(const char &#42;key, char &#42;out, size_t cap) {</code>                   |
| `log_stats`            | function |  954 | <code>static long log_stats(const char &#42;path, long &#42;last_ts) {</code>                         |
| `jev_doctor`           | function |  970 | <code>static int jev_doctor(Cg &#42;cg, bool probe, bool json) {</code>                               |
| `jev_log_cmd`          | function | 1062 | <code>static int jev_log_cmd(Cg &#42;cg, int limit, bool json) {</code>                               |
| `cmd_jev`              | function | 1134 | <code>int cmd_jev(Cg &#42;cg, int argc, char &#42;&#42;argv, bool json) {</code>                      |
| `JEV_TAIL_BYTES`       | macro    | 1163 | <code>#define JEV_TAIL_BYTES 4096</code>                                                              |
| `JEV_TAIL_LINES`       | macro    | 1164 | <code>#define JEV_TAIL_LINES 40</code>                                                                |
| `JEV_MAX_RANK`         | macro    | 1165 | <code>#define JEV_MAX_RANK 50</code>                                                                  |
| `jev_advisory_ready`   | function | 1167 | <code>bool jev_advisory_ready(const char &#42;what) {</code>                                          |
| `jev_advisory_failed`  | function | 1174 | <code>static void jev_advisory_failed(const char &#42;what, const JevResult &#42;r) {</code>          |
| `jev_tail`             | function | 1181 | <code>static void jev_tail(const char &#42;s, StrBuf &#42;out) {</code>                               |
| `answer_confidence`    | function | 1200 | <code>static double answer_confidence(const JevAnswer &#42;a) {</code>                                |
| `sb_confidence`        | function | 1204 | <code>static void sb_confidence(StrBuf &#42;b, double c) {</code>                                     |
| `jev_triage_failure`   | function | 1208 | <code>int jev_triage_failure(Cg &#42;cg, const char &#42;output, JevTriage &#42;out) {</code>         |
| `rank_sort`            | function | 1285 | <code>static void rank_sort(JevFinding &#42;v, int n) {</code>                                        |
| `jev_rank_findings`    | function | 1294 | <code>int jev_rank_findings(Cg &#42;cg, JevFinding &#42;v, int n) {</code>                            |
| `jev_pr_readiness`     | function | 1360 | <code>int jev_pr_readiness(Cg &#42;cg, const char &#42;state_json, JevReadiness &#42;out) {</code>    |
| `jev_report_error`     | function | 1396 | <code>int jev_report_error(const JevResult &#42;r, const char &#42;what) {</code>                     |

## core/native/src/kvx.c

[Open source](../../../tools/hyperfide/core/native/src/kvx.c)

| Indexed name      | Kind     | Line | Source declaration                                                                                               |
| ----------------- | -------- | ---: | ---------------------------------------------------------------------------------------------------------------- |
| `strip_comment`   | function |   19 | <code>static size_t strip_comment(char &#42;s, size_t n) {</code>                                                |
| `trim`            | function |   30 | <code>static char &#42;trim(char &#42;s) {</code>                                                                |
| `kvx_add_section` | function |   98 | <code>static int kvx_add_section(Kvx &#42;k, const char &#42;sec) {</code>                                       |
| `kvx_parse`       | function |  121 | <code>Kvx &#42;kvx_parse(const char &#42;path) {</code>                                                          |
| `kvx_free`        | function |  183 | <code>void kvx_free(Kvx &#42;k) {</code>                                                                         |
| `kvx_has`         | function |  198 | <code>bool kvx_has(const Kvx &#42;k, const char &#42;sec) {</code>                                               |
| `kvx_raw`         | function |  202 | <code>const char &#42;kvx_raw(const Kvx &#42;k, const char &#42;sec, const char &#42;key) {</code>               |
| `env_name`        | function |  208 | <code>static bool env_name(const char &#42;s, size_t n) {</code>                                                 |
| `interp_unquote`  | function |  216 | <code>static char &#42;interp_unquote(const char &#42;raw) {</code>                                              |
| `kvx_str`         | function |  242 | <code>char &#42;kvx_str(const Kvx &#42;k, const char &#42;sec, const char &#42;key) {</code>                     |
| `kvx_long`        | function |  248 | <code>long kvx_long(const Kvx &#42;k, const char &#42;sec, const char &#42;key, long dflt) {</code>              |
| `kvx_bool`        | function |  258 | <code>bool kvx_bool(const Kvx &#42;k, const char &#42;sec, const char &#42;key, bool dflt) {</code>              |
| `kvx_list`        | function |  267 | <code>int kvx_list(const Kvx &#42;k, const char &#42;sec, const char &#42;key, char &#42;&#42;&#42;out) {</code> |
| `kvx_keys`        | function |  308 | <code>int kvx_keys(const Kvx &#42;k, const char &#42;sec, const char &#42;&#42;&#42;out) {</code>                |
| `kvx_subsections` | function |  320 | <code>int kvx_subsections(const Kvx &#42;k, const char &#42;prefix, char &#42;&#42;&#42;out) {</code>            |
| `seg_int`         | function |  341 | <code>static bool seg_int(const char &#42;s, size_t n, long &#42;out) {</code>                                   |
| `dotted_cmp`      | function |  355 | <code>static int dotted_cmp(const void &#42;pa, const void &#42;pb) {</code>                                     |
| `kvx_sort_dotted` | function |  374 | <code>void kvx_sort_dotted(char &#42;&#42;ids, int n) {</code>                                                   |
| `kvx_lock`        | function |  383 | <code>static int kvx_lock(const char &#42;path) {</code>                                                         |
| `kvx_unlock`      | function |  391 | <code>static void kvx_unlock(int fd) {</code>                                                                    |
| `kvx_status_hook` | function |  395 | <code>void (&#42;kvx_status_hook)(const char &#42;path, const char &#42;section,</code>                          |
| `old_scalar`      | function |  399 | <code>static void old_scalar(const char &#42;after_eq, char &#42;out, size_t cap) {</code>                       |
| `kvx_set_status`  | function |  409 | <code>int kvx_set_status(const char &#42;path, const char &#42;section, const char &#42;value) {</code>          |
| `sb_kvx_string`   | function |  471 | <code>static void sb_kvx_string(StrBuf &#42;b, const char &#42;value) {</code>                                   |
| `kvx_set_value`   | function |  492 | <code>static int kvx_set_value(const char &#42;path, const char &#42;section, const char &#42;key,</code>        |
| `kvx_set_string`  | function |  603 | <code>int kvx_set_string(const char &#42;path, const char &#42;section, const char &#42;key,</code>              |
| `kvx_set_raw`     | function |  608 | <code>int kvx_set_raw(const char &#42;path, const char &#42;section, const char &#42;key,</code>                 |

## core/native/src/memory.c

[Open source](../../../tools/hyperfide/core/native/src/memory.c)

| Indexed name             | Kind     | Line | Source declaration                                                                                           |
| ------------------------ | -------- | ---: | ------------------------------------------------------------------------------------------------------------ |
| `hf_decision_advice`     | function |   15 | <code>extern int hf_decision_advice(int purpose, const char &#42;observation, char &#42;&#42;advice);</code> |
| `hf_advice_unit`         | function |   16 | <code>extern int hf_advice_unit(const char &#42;obj, const char &#42;key, double &#42;out);</code>           |
| `memory_open_quiet`      | function |   20 | <code>bool memory_open_quiet(Cg &#42;g) {</code>                                                             |
| `bind_opt`               | function |   26 | <code>static void bind_opt(sqlite3_stmt &#42;st, int i, const char &#42;v) {</code>                          |
| `memory_add`             | function |   31 | <code>long memory_add(Cg &#42;cg, const char &#42;type, const char &#42;task, const char &#42;body,</code>   |
| `memory_add_at`          | function |   39 | <code>long memory_add_at(Cg &#42;cg, long created, const char &#42;branch, const char &#42;type,</code>      |
| `fts_query`              | function |   98 | <code>static char &#42;fts_query(const char &#42;q) {</code>                                                 |
| `col_dup`                | function |  118 | <code>static char &#42;col_dup(sqlite3_stmt &#42;st, int i) {</code>                                         |
| `mem_row`                | function |  127 | <code>static void mem_row(sqlite3_stmt &#42;st, Memory &#42;m) {</code>                                      |
| `memory_get`             | function |  151 | <code>bool memory_get(Cg &#42;cg, long id, Memory &#42;out) {</code>                                         |
| `mem_scope`              | function |  165 | <code>static const char &#42;mem_scope(Cg &#42;cg, char &#42;out, size_t cap) {</code>                       |
| `MEM_BRANCH_CTE`         | macro    |  186 | <code>#define MEM_BRANCH_CTE \</code>                                                                        |
| `MEM_BRANCH_WHERE`       | macro    |  190 | <code>#define MEM_BRANCH_WHERE \</code>                                                                      |
| `SUPERSEDED_RANK`        | macro    |  197 | <code>#define SUPERSEDED_RANK \</code>                                                                       |
| `memory_query`           | function |  200 | <code>int memory_query(Cg &#42;cg, const char &#42;query, const char &#42;task,</code>                       |
| `memory_clear`           | function |  266 | <code>void memory_clear(Memory &#42;m) {</code>                                                              |
| `memory_free`            | function |  273 | <code>void memory_free(Memory &#42;v, int n) {</code>                                                        |
| `memory_json`            | function |  278 | <code>void memory_json(const Memory &#42;m, StrBuf &#42;b) {</code>                                          |
| `memory_print_brief`     | function |  304 | <code>void memory_print_brief(const Memory &#42;m, const char &#42;indent) {</code>                          |
| `cmd_remember`           | function |  317 | <code>int cmd_remember(Cg &#42;cg, const char &#42;text, const char &#42;type, const char &#42;task,</code>  |
| `cmd_remember_ex`        | function |  325 | <code>int cmd_remember_ex(Cg &#42;cg, const char &#42;text, const char &#42;type,</code>                     |
| `cmd_recall`             | function |  398 | <code>int cmd_recall(Cg &#42;cg, const char &#42;query, const char &#42;task, const char &#42;type,</code>   |
| `cmd_forget`             | function |  481 | <code>int cmd_forget(Cg &#42;cg, const char &#42;idstr) {</code>                                             |
| `memory_supersede`       | function |  506 | <code>int memory_supersede(Cg &#42;cg, long old_id, long new_id) {</code>                                    |
| `cmd_recall_near`        | function |  529 | <code>int cmd_recall_near(Cg &#42;cg, const char &#42;path, int limit, bool json) {</code>                   |
| `cmd_memory_compact`     | function |  573 | <code>int cmd_memory_compact(Cg &#42;cg, bool dry_run, bool json) {</code>                                   |
| `CLASS_KEYS`             | variable |  629 | <code>static const char &#42;const CLASS_KEYS&#91;&#93; = {</code>                                           |
| `CLASS_DESCS`            | variable |  632 | <code>static const char &#42;const CLASS_DESCS&#91;&#93; = {</code>                                          |
| `NCLASSES`               | macro    |  642 | <code>#define NCLASSES ((int)(sizeof CLASS_KEYS / sizeof CLASS_KEYS&#91;0&#93;))</code>                      |
| `CLASSIFY_DEFAULT_LIMIT` | macro    |  643 | <code>#define CLASSIFY_DEFAULT_LIMIT 50</code>                                                               |
| `state_field`            | function |  645 | <code>static void state_field(StrBuf &#42;b, const char &#42;key, const char &#42;val) {</code>              |
| `classify_state`         | function |  653 | <code>static char &#42;classify_state(const Memory &#42;m) {</code>                                          |
| `classify_one`           | function |  669 | <code>static int classify_one(Cg &#42;cg, Memory &#42;m, double &#42;reusable) {</code>                      |
| `classify_store`         | function |  734 | <code>static void classify_store(Cg &#42;cg, const Memory &#42;m) {</code>                                   |
| `classify_select`        | function |  747 | <code>static int classify_select(Cg &#42;cg, const char &#42;sel, int limit, Memory &#42;&#42;out) {</code>  |
| `cmd_memory_classify`    | function |  784 | <code>int cmd_memory_classify(Cg &#42;cg, const char &#42;sel, int limit, bool json) {</code>                |
| `memory_promote_branch`  | function |  864 | <code>int memory_promote_branch(Cg &#42;cg, const char &#42;from, const char &#42;to) {</code>               |
| `PORT_FORMAT`            | macro    |  902 | <code>#define PORT_FORMAT "codify-memories"</code>                                                           |
| `PORT_VERSION`           | macro    |  903 | <code>#define PORT_VERSION 1</code>                                                                          |
| `memory_content_id`      | function |  908 | <code>void memory_content_id(const char &#42;type, const char &#42;task, const char &#42;body,</code>        |
| `db_has`                 | function |  920 | <code>static bool db_has(sqlite3 &#42;db, const char &#42;sql, const char &#42;a,</code>                     |
| `db_has_column`          | function |  931 | <code>static bool db_has_column(sqlite3 &#42;db, const char &#42;table, const char &#42;col) {</code>        |
| `db_has_table`           | function |  936 | <code>static bool db_has_table(sqlite3 &#42;db, const char &#42;table) {</code>                              |
| `port_str`               | function |  941 | <code>static void port_str(StrBuf &#42;b, const char &#42;key, const char &#42;v) {</code>                   |
| `port_export`            | function |  950 | <code>static int port_export(sqlite3 &#42;db, const char &#42;project,</code>                                |
| `base_name`              | function | 1040 | <code>static const char &#42;base_name(const char &#42;path) {</code>                                        |
| `cmd_memory_export`      | function | 1045 | <code>int cmd_memory_export(Cg &#42;cg, const MemExportOpts &#42;o, bool json) {</code>                      |
| `jv_value`               | function | 1083 | <code>static const char &#42;jv_value(const char &#42;p, int depth);</code>                                  |
| `jv_ws`                  | function | 1085 | <code>static const char &#42;jv_ws(const char &#42;p) {</code>                                               |
| `jv_string`              | function | 1090 | <code>static const char &#42;jv_string(const char &#42;p) {</code>                                           |
| `jv_digits`              | function | 1109 | <code>static const char &#42;jv_digits(const char &#42;p) {</code>                                           |
| `jv_number`              | function | 1115 | <code>static const char &#42;jv_number(const char &#42;p) {</code>                                           |
| `jv_value`               | function | 1127 | <code>static const char &#42;jv_value(const char &#42;p, int depth) {</code>                                 |
| `json_object_ok`         | function | 1154 | <code>static bool json_object_ok(const char &#42;s) {</code>                                                 |
| `port_get_str`           | function | 1162 | <code>static char &#42;port_get_str(const char &#42;obj, const char &#42;key, bool &#42;bad) {</code>        |
| `PortRec`                | typedef  | 1171 | <code>typedef struct {</code>                                                                                |
| `port_rec_free`          | function | 1186 | <code>static void port_rec_free(PortRec &#42;r) {</code>                                                     |
| `port_type_ok`           | function | 1195 | <code>static bool port_type_ok(const char &#42;t) {</code>                                                   |
| `port_parse_line`        | function | 1205 | <code>static const char &#42;port_parse_line(const char &#42;s, PortRec &#42;r) {</code>                     |
| `PortIn`                 | typedef  | 1245 | <code>typedef struct {</code>                                                                                |
| `port_bad_line`          | function | 1253 | <code>static void port_bad_line(PortIn &#42;in, int line, const char &#42;why) {</code>                      |
| `port_parse`             | function | 1265 | <code>static int port_parse(char &#42;text, PortIn &#42;in, char &#42;err, size_t errcap) {</code>           |
| `CidRow`                 | typedef  | 1330 | <code>typedef struct { char cid&#91;65&#93;; long row; } CidRow;</code>                                      |
| `cid_cmp`                | function | 1332 | <code>static int cid_cmp(const void &#42;a, const void &#42;b) {</code>                                      |
| `cid_row`                | function | 1339 | <code>static long cid_row(const CidRow &#42;v, int n, const char &#42;cid) {</code>                          |
| `target_cids`            | function | 1351 | <code>static int target_cids(Cg &#42;cg, CidRow &#42;&#42;out) {</code>                                      |
| `port_from_dir`          | function | 1371 | <code>static int port_from_dir(Cg &#42;cg, const char &#42;dir, StrBuf &#42;out,</code>                      |
| `port_step`              | function | 1407 | <code>static bool port_step(Cg &#42;cg, sqlite3_stmt &#42;st, char &#42;err, size_t errcap) {</code>         |
| `port_insert`            | function | 1415 | <code>static bool port_insert(Cg &#42;cg, PortRec &#42;r, char &#42;err, size_t errcap) {</code>             |
| `port_relink`            | function | 1445 | <code>static int port_relink(Cg &#42;cg, PortIn &#42;in, const CidRow &#42;tv, int tn,</code>                |
| `port_plan`              | function | 1500 | <code>static void port_plan(Cg &#42;cg, PortIn &#42;in, const MemImportOpts &#42;o,</code>                   |
| `port_read_input`        | function | 1556 | <code>static int port_read_input(Cg &#42;cg, const MemImportOpts &#42;o, char &#42;&#42;text,</code>         |
| `cmd_memory_import`      | function | 1578 | <code>int cmd_memory_import(Cg &#42;cg, const MemImportOpts &#42;o, bool json) {</code>                      |

## core/native/src/recap.c

[Open source](../../../tools/hyperfide/core/native/src/recap.c)

| Indexed name            | Kind     | Line | Source declaration                                                                                                                             |
| ----------------------- | -------- | ---: | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `RECAP_DECIDE_MODEL`    | macro    |   48 | <code>#define RECAP_DECIDE_MODEL "openrouter/upstage/solar-decide"</code>                                                                      |
| `RECAP_DECIDE_ENDPOINT` | macro    |   49 | <code>#define RECAP_DECIDE_ENDPOINT "https://gateway.centra.ag/v1/systemone"</code>                                                            |
| `RECAP_CHUNK`           | macro    |   50 | <code>#define RECAP_CHUNK 6 /&#42; statements per call, 3 questions each; the</code>                                                           |
| `RECAP_CHUNK_CHARS`     | macro    |   53 | <code>#define RECAP_CHUNK_CHARS 9000</code>                                                                                                    |
| `RECAP_PARALLEL`        | macro    |   54 | <code>#define RECAP_PARALLEL 6 /&#42; decision calls in flight at once &#42;/</code>                                                           |
| `RECAP_MAX_SNIPPETS`    | macro    |   55 | <code>#define RECAP_MAX_SNIPPETS 160</code>                                                                                                    |
| `RECAP_USER_MAX`        | macro    |   56 | <code>#define RECAP_USER_MAX 800</code>                                                                                                        |
| `RECAP_ASSISTANT_MAX`   | macro    |   57 | <code>#define RECAP_ASSISTANT_MAX 400</code>                                                                                                   |
| `RECAP_TOOL_MAX`        | macro    |   58 | <code>#define RECAP_TOOL_MAX 200</code>                                                                                                        |
| `RECAP_KEEP_USERS`      | macro    |   59 | <code>#define RECAP_KEEP_USERS 50</code>                                                                                                       |
| `RECAP_MIN_TRUE`        | macro    |   60 | <code>#define RECAP_MIN_TRUE 0.5</code>                                                                                                        |
| `RECAP_MIN_NEED`        | macro    |   61 | <code>#define RECAP_MIN_NEED 0.5</code>                                                                                                        |
| `Snip`                  | typedef  |   65 | <code>typedef struct {</code>                                                                                                                  |
| `Session`               | typedef  |   73 | <code>typedef struct {</code>                                                                                                                  |
| `KIND_KEYS`             | variable |   85 | <code>static const char &#42;const KIND_KEYS&#91;&#93; = {</code>                                                                              |
| `KIND_DESCS`            | variable |   88 | <code>static const char &#42;const KIND_DESCS&#91;&#93; = {</code>                                                                             |
| `NKINDS`                | macro    |   98 | <code>#define NKINDS ((int)(sizeof KIND_KEYS / sizeof KIND_KEYS&#91;0&#93;))</code>                                                            |
| `KIND_NOISE`            | macro    |   99 | <code>#define KIND_NOISE (NKINDS - 1)</code>                                                                                                   |
| `sess_push`             | function |  101 | <code>static void sess_push(Session &#42;s, char role, const char &#42;text) {</code>                                                          |
| `sess_file`             | function |  138 | <code>static void sess_file(Session &#42;s, const char &#42;path) {</code>                                                                     |
| `sess_free`             | function |  146 | <code>static void sess_free(Session &#42;s) {</code>                                                                                           |
| `strip_blocks`          | function |  158 | <code>static void strip_blocks(char &#42;t, const char &#42;tag) {</code>                                                                      |
| `user_words`            | function |  175 | <code>static char &#42;user_words(const char &#42;raw) {</code>                                                                                |
| `assistant_words`       | function |  211 | <code>static void assistant_words(Session &#42;s, const char &#42;text) {</code>                                                               |
| `EMIT`                  | macro    |  217 | <code>#define EMIT(str) do { sess_push(s, 'a', (str)); if (clean.len) sb_putc(&amp;clean, ' '); sb_puts(&amp;clean, (str)); } while (0)</code> |
| `command_matters`       | function |  259 | <code>static bool command_matters(const char &#42;cmd) {</code>                                                                                |
| `set_date`              | function |  267 | <code>static void set_date(Session &#42;s, const char &#42;ts) {</code>                                                                        |
| `claude_content`        | function |  274 | <code>static void claude_content(Session &#42;s, const char &#42;msg, bool user) {</code>                                                      |
| `parse_claude`          | function |  323 | <code>static int parse_claude(Session &#42;s, FILE &#42;f) {</code>                                                                            |
| `codex_texts`           | function |  351 | <code>static void codex_texts(Session &#42;s, const char &#42;content, char role) {</code>                                                     |
| `parse_codex`           | function |  365 | <code>static int parse_codex(Session &#42;s, FILE &#42;f) {</code>                                                                             |
| `Sessions`              | typedef  |  429 | <code>typedef struct { Session &#42;v; int n, cap; } Sessions;</code>                                                                          |
| `sessions_add`          | function |  431 | <code>static void sessions_add(Sessions &#42;ss, const Session &#42;s) {</code>                                                                |
| `file_mtime`            | function |  439 | <code>static long file_mtime(const char &#42;path) {</code>                                                                                    |
| `transcript_dirs`       | function |  451 | <code>void transcript_dirs(const char &#42;root, char &#42;claude, size_t ccap,</code>                                                         |
| `transcript_codex_head` | function |  474 | <code>bool transcript_codex_head(const char &#42;path, char &#42;cwd, size_t ccap,</code>                                                      |
| `claude_dir`            | function |  505 | <code>static void claude_dir(const char &#42;root, char &#42;out, size_t cap) {</code>                                                         |
| `find_claude`           | function |  509 | <code>static void find_claude(const char &#42;root, long oldest, Sessions &#42;out) {</code>                                                   |
| `codex_owned`           | function |  530 | <code>static bool codex_owned(const char &#42;path, const char &#42;root) {</code>                                                             |
| `find_codex_in`         | function |  538 | <code>static void find_codex_in(const char &#42;dir, const char &#42;root, long oldest,</code>                                                 |
| `find_codex`            | function |  567 | <code>static void find_codex(const char &#42;root, long oldest, Sessions &#42;out) {</code>                                                    |
| `cmp_newest`            | function |  573 | <code>static int cmp_newest(const void &#42;a, const void &#42;b) {</code>                                                                     |
| `cmp_oldest`            | function |  578 | <code>static int cmp_oldest(const void &#42;a, const void &#42;b) { return -cmp_newest(a, b); }</code>                                         |
| `sess_cap`              | function |  582 | <code>static void sess_cap(Session &#42;s, int cap) {</code>                                                                                   |
| `Recap`                 | typedef  |  602 | <code>typedef struct {</code>                                                                                                                  |
| `recap_config`          | function |  611 | <code>static void recap_config(const Cg &#42;cg, Recap &#42;r) {</code>                                                                        |
| `role_name`             | function |  624 | <code>static const char &#42;role_name(char r) {</code>                                                                                        |
| `chunk_state`           | function |  631 | <code>static void chunk_state(const Session &#42;s, int from, int to, StrBuf &#42;b) {</code>                                                  |
| `cache_path`            | function |  655 | <code>static void cache_path(const Recap &#42;r, const char &#42;state, char &#42;out, size_t cap) {</code>                                    |
| `cache_read`            | function |  664 | <code>static bool cache_read(const char &#42;path, Session &#42;s, int from, int to) {</code>                                                  |
| `cache_write`           | function |  683 | <code>static void cache_write(const char &#42;path, const Session &#42;s, int from, int to) {</code>                                           |
| `hf_decision_advice`    | function |  693 | <code>int hf_decision_advice(int purpose, const char &#42;observation, char &#42;&#42;advice);</code>                                          |
| `hf_advice_unit`        | function |  694 | <code>int hf_advice_unit(const char &#42;obj, const char &#42;key, double &#42;out);</code>                                                    |
| `recap_advice_apply`    | function |  700 | <code>static int recap_advice_apply(const char &#42;advice, Session &#42;s, int from, int to) {</code>                                         |
| `decide_chunk`          | function |  734 | <code>static int decide_chunk(Cg &#42;cg, Recap &#42;r, Session &#42;s, int from, int to) {</code>                                             |
| `Chunk`                 | typedef  |  829 | <code>typedef struct { int sess, from, to; } Chunk;</code>                                                                                     |
| `chunks_of`             | function |  831 | <code>static int chunks_of(const Sessions &#42;ss, Chunk &#42;&#42;out) {</code>                                                               |
| `chunk_cached`          | function |  854 | <code>static bool chunk_cached(Recap &#42;r, Session &#42;s, const Chunk &#42;c) {</code>                                                      |
| `decide_parallel`       | function |  871 | <code>static void decide_parallel(Cg &#42;cg, Recap &#42;r, Sessions &#42;ss, const Chunk &#42;v, int n, int par) {</code>                     |
| `decide_all`            | function |  889 | <code>static void decide_all(Cg &#42;cg, Recap &#42;r, Sessions &#42;ss) {</code>                                                              |
| `Pick`                  | typedef  |  911 | <code>typedef struct { int sess, idx; double score; } Pick;</code>                                                                             |
| `snip_score`            | function |  913 | <code>static double snip_score(const Snip &#42;p) {</code>                                                                                     |
| `cmp_pick`              | function |  919 | <code>static int cmp_pick(const void &#42;a, const void &#42;b) {</code>                                                                       |
| `select_picks`          | function |  926 | <code>static int select_picks(Sessions &#42;ss, long budget) {</code>                                                                          |
| `render_decided`        | function |  951 | <code>static void render_decided(const Sessions &#42;ss, const Recap &#42;r, StrBuf &#42;b) {</code>                                           |
| `run_capture`           | function |  979 | <code>static void run_capture(const char &#42;cmd, StrBuf &#42;out) {</code>                                                                   |
| `repo_facts`            | function |  988 | <code>static void repo_facts(Cg &#42;cg, int since_days, StrBuf &#42;b) {</code>                                                               |
| `write_brief`           | function | 1079 | <code>static char &#42;write_brief(Cg &#42;cg, Recap &#42;r, const Sessions &#42;ss, const char &#42;facts,</code>                             |
| `agent_wanted`          | function | 1123 | <code>static bool agent_wanted(const char &#42;list, const char &#42;agent) {</code>                                                           |
| `cmd_recap`             | function | 1128 | <code>int cmd_recap(Cg &#42;cg, const RecapOpts &#42;o) {</code>                                                                               |
| `hf_uuid_generate`      | function | 1275 | <code>extern int hf_uuid_generate(char output&#91;37&#93;);</code>                                                                             |
| `HF_ADVICE_BUDGET_MS`   | macro    | 1277 | <code>#define HF_ADVICE_BUDGET_MS 2000</code>                                                                                                  |
| `hf_decision_phase`     | function | 1280 | <code>static char &#42;hf_decision_phase(int purpose, int phase, const char &#42;input,</code>                                                 |
| `hf_decision_advice`    | function | 1319 | <code>int hf_decision_advice(int purpose, const char &#42;observation, char &#42;&#42;advice) {</code>                                         |
| `hf_advice_unit`        | function | 1374 | <code>int hf_advice_unit(const char &#42;obj, const char &#42;key, double &#42;out) {</code>                                                   |

## core/native/src/skills.c

[Open source](../../../tools/hyperfide/core/native/src/skills.c)

| Indexed name        | Kind     | Line | Source declaration                                                                                 |
| ------------------- | -------- | ---: | -------------------------------------------------------------------------------------------------- |
| `SKILL_MARKER`      | macro    |   23 | <code>#define SKILL_MARKER "codify-owned: memory-skill v1"</code>                                  |
| `SKILL_SLUG_MAX`    | macro    |   24 | <code>#define SKILL_SLUG_MAX 48</code>                                                             |
| `SkillFile`         | typedef  |   27 | <code>typedef struct { char rel&#91;1200&#93;; char slug&#91;80&#93;; long id; } SkillFile;</code> |
| `skill_title`       | function |   30 | <code>static void skill_title(const Memory &#42;m, char &#42;out, size_t cap) {</code>             |
| `skill_slug`        | function |   45 | <code>static void skill_slug(const Memory &#42;m, char &#42;out, size_t cap) {</code>              |
| `yaml_str`          | function |   63 | <code>static void yaml_str(StrBuf &#42;b, const char &#42;s, size_t max) {</code>                  |
| `skill_memory_of`   | function |   74 | <code>static long skill_memory_of(const char &#42;body) {</code>                                   |
| `skill_body`        | function |   84 | <code>static char &#42;skill_body(const Memory &#42;m) {</code>                                    |
| `skill_render`      | function |  140 | <code>int skill_render(Cg &#42;cg, const Memory &#42;m, char &#42;path_out, size_t cap) {</code>   |
| `skill_scan`        | function |  183 | <code>static int skill_scan(Cg &#42;cg, SkillFile &#42;&#42;out) {</code>                          |
| `skill_current`     | function |  222 | <code>static bool skill_current(Cg &#42;cg, const SkillFile &#42;f, const Memory &#42;m) {</code>  |
| `skill_findings`    | function |  233 | <code>int skill_findings(Cg &#42;cg, char &#42;&#42;&#42;out) {</code>                             |
| `skill_candidates`  | function |  266 | <code>static int skill_candidates(Cg &#42;cg, Memory &#42;&#42;out) {</code>                       |
| `skill_path_for`    | function |  283 | <code>static const char &#42;skill_path_for(const SkillFile &#42;v, int n, long id) {</code>       |
| `skill_row_json`    | function |  288 | <code>static void skill_row_json(StrBuf &#42;b, const Memory &#42;m, const char &#42;path,</code>  |
| `skills_list`       | function |  308 | <code>static int skills_list(Cg &#42;cg, bool json) {</code>                                       |
| `skills_promote`    | function |  381 | <code>static int skills_promote(Cg &#42;cg, const char &#42;idstr, bool json) {</code>             |
| `skills_render_all` | function |  432 | <code>static int skills_render_all(Cg &#42;cg, bool json) {</code>                                 |
| `cmd_skills`        | function |  491 | <code>int cmd_skills(Cg &#42;cg, int argc, char &#42;&#42;argv, bool json) {</code>                |

## core/native/src/spec.c

[Open source](../../../tools/hyperfide/core/native/src/spec.c)

| Indexed name                     | Kind     | Line | Source declaration                                                                                                               |
| -------------------------------- | -------- | ---: | -------------------------------------------------------------------------------------------------------------------------------- |
| `hf_client_authority_guard`      | function |    2 | <code>extern int hf_client_authority_guard(const char &#42;operation, FILE &#42;output);</code>                                  |
| `hf_client_authority_guard_args` | function |    3 | <code>extern int hf_client_authority_guard_args(const char &#42;operation,int argc,char &#42;&#42;argv,FILE &#42;output);</code> |
| `hf_uuid_generate`               | function |    5 | <code>extern int hf_uuid_generate(char output&#91;37&#93;);</code>                                                               |
| `hyperfide_owned_write`          | function |   32 | <code>extern int hyperfide_owned_write(const char &#42;,const char &#42;,size_t,const char &#42;,bool);</code>                   |
| `SPEC_BANNER_DFLT`               | macro    |   34 | <code>#define SPEC_BANNER_DFLT "GENERATED by hyperfide spec render — DO NOT EDIT."</code>                                        |
| `S`                              | function |   39 | <code>static char &#42;S(const Kvx &#42;k, const char &#42;sec, const char &#42;key) {</code>                                    |
| `raw_is_list`                    | function |   44 | <code>static bool raw_is_list(const Kvx &#42;k, const char &#42;sec, const char &#42;key) {</code>                               |
| `count_dots`                     | function |   51 | <code>static int count_dots(const char &#42;s) {</code>                                                                          |
| `checkbox`                       | function |   72 | <code>static const char &#42;checkbox(const char &#42;status) {</code>                                                           |
| `uint_or`                        | function |   81 | <code>static unsigned long uint_or(const Kvx &#42;k, const char &#42;sec, const char &#42;key,</code>                            |
| `FOR_KV`                         | macro    |   93 | <code>#define FOR_KV(k, secname, prefix, idx) \</code>                                                                           |
| `entry_val`                      | function |   98 | <code>static char &#42;entry_val(const Kvx &#42;k, int i) { /&#42; interpolated &#42;/</code>                                    |
| `spec_find_root`                 | function |  102 | <code>static int spec_find_root(char &#42;out, size_t cap) {</code>                                                              |
| `read_include`                   | function |  107 | <code>static char &#42;read_include(const char &#42;fdir, const char &#42;rel) {</code>                                          |
| `Writer`                         | typedef  |  118 | <code>typedef struct {</code>                                                                                                    |
| `writer_write_owned`             | function |  123 | <code>static int writer_write_owned(Writer &#42;w, const char &#42;path, const char &#42;content) {</code>                       |
| `writer_write`                   | function |  147 | <code>static int writer_write(Writer &#42;w,const char &#42;path,const char &#42;content) {</code>                               |
| `build_brief`                    | function |  163 | <code>static char &#42;build_brief(const Kvx &#42;wf, const char &#42;specrel) {</code>                                          |
| `adapter_render`                 | function |  212 | <code>static char &#42;adapter_render(const char &#42;label, const char &#42;brief,</code>                                       |
| `render_requirements`            | function |  234 | <code>static char &#42;render_requirements(const Kvx &#42;f, const char &#42;banner) {</code>                                    |
| `render_design`                  | function |  267 | <code>static char &#42;render_design(const Kvx &#42;f, const char &#42;fdir, const char &#42;banner,</code>                      |
| `render_wave_graph`              | function |  321 | <code>static void render_wave_graph(const Kvx &#42;f, char &#42;&#42;ids, int nids, StrBuf &#42;b) {</code>                      |
| `render_tasks`                   | function |  368 | <code>static char &#42;render_tasks(const Kvx &#42;f, const char &#42;fdir, const char &#42;banner,</code>                       |
| `OutFile`                        | typedef  |  486 | <code>typedef struct { char &#42;path, &#42;content; } OutFile;</code>                                                           |
| `outfile_cmp`                    | function |  488 | <code>static int outfile_cmp(const void &#42;a, const void &#42;b) {</code>                                                      |
| `strp_cmp`                       | function |  492 | <code>static int strp_cmp(const void &#42;a, const void &#42;b) {</code>                                                         |
| `discover_features`              | function |  496 | <code>static int discover_features(const char &#42;specdir, char &#42;&#42;&#42;out) {</code>                                    |
| `render_feature`                 | function |  520 | <code>static int render_feature(const char &#42;root, const char &#42;specdir,</code>                                            |
| `spec_render`                    | function |  575 | <code>static int spec_render(const char &#42;root, bool check, bool quiet) {</code>                                              |
| `spec_render_json`               | function |  643 | <code>static int spec_render_json(const char &#42;root, bool check) {</code>                                                     |
| `Spec`                           | typedef  |  652 | <code>typedef struct {</code>                                                                                                    |
| `spec_close`                     | function |  661 | <code>static void spec_close(Spec &#42;s) {</code>                                                                               |
| `spec_load`                      | function |  669 | <code>static int spec_load(Spec &#42;s, const char &#42;root_ov, const char &#42;feature_ov,</code>                              |
| `task_sec`                       | function |  714 | <code>static void task_sec(char &#42;buf, size_t cap, const char &#42;id) {</code>                                               |
| `task_exists`                    | function |  718 | <code>static bool task_exists(const Spec &#42;s, const char &#42;id) {</code>                                                    |
| `task_status`                    | function |  725 | <code>static char &#42;task_status(const Spec &#42;s, const char &#42;id) {</code>                                               |
| `spec_mode_is`                   | function |  731 | <code>static bool spec_mode_is(const Spec &#42;s, const char &#42;want) {</code>                                                 |
| `spec_prod_mode`                 | function |  739 | <code>static bool spec_prod_mode(const Spec &#42;s) {</code>                                                                     |
| `spec_parallel_mode`             | function |  745 | <code>static bool spec_parallel_mode(const Spec &#42;s) {</code>                                                                 |
| `task_is_leaf`                   | function |  749 | <code>static bool task_is_leaf(const Spec &#42;s, const char &#42;id);</code>                                                    |
| `spec_docs_mode`                 | function |  754 | <code>static char &#42;spec_docs_mode(const Spec &#42;s) {</code>                                                                |
| `spec_all_tasks_qualified`       | function |  761 | <code>static bool spec_all_tasks_qualified(const Spec &#42;s) {</code>                                                           |
| `spec_docs_stage`                | function |  777 | <code>static char &#42;spec_docs_stage(const Spec &#42;s) {</code>                                                               |
| `spec_docs_ready`                | function |  795 | <code>static bool spec_docs_ready(const Spec &#42;s) {</code>                                                                    |
| `spec_docs_set_status`           | function |  802 | <code>static int spec_docs_set_status(Spec &#42;s, const char &#42;status) {</code>                                              |
| `json_docs_task`                 | function |  810 | <code>static void json_docs_task(StrBuf &#42;b, const Spec &#42;s) {</code>                                                      |
| `print_docs_task`                | function |  825 | <code>static void print_docs_task(const Spec &#42;s) {</code>                                                                    |
| `task_is_leaf`                   | function |  837 | <code>static bool task_is_leaf(const Spec &#42;s, const char &#42;id) {</code>                                                   |
| `task_satisfies_requires`        | function |  844 | <code>static bool task_satisfies_requires(const Spec &#42;s, const char &#42;id) {</code>                                        |
| `task_unmet`                     | function |  853 | <code>static int task_unmet(const Spec &#42;s, const char &#42;id, char &#42;&#42;unmet, int cap) {</code>                       |
| `task_eligible`                  | function |  869 | <code>static bool task_eligible(const Spec &#42;s, const char &#42;id) {</code>                                                  |
| `spec_next_id`                   | function |  883 | <code>static const char &#42;spec_next_id(const Spec &#42;s) {</code>                                                            |
| `clause_text`                    | function |  897 | <code>static char &#42;clause_text(const Spec &#42;s, const char &#42;clause) {</code>                                           |
| `print_task`                     | function |  916 | <code>static void print_task(const Spec &#42;s, const char &#42;id) {</code>                                                     |
| `json_task`                      | function |  974 | <code>static void json_task(const Spec &#42;s, const char &#42;id, StrBuf &#42;b) {</code>                                       |
| `spec_current`                   | function | 1042 | <code>static const char &#42;spec_current(const Spec &#42;s) {</code>                                                            |
| `spec_in_progress_count`         | function | 1052 | <code>static int spec_in_progress_count(const Spec &#42;s) {</code>                                                              |
| `spec_attempt_sweep`             | function | 1069 | <code>static void spec_attempt_sweep(Cg &#42;g) {</code>                                                                         |
| `spec_claim_begin`               | function | 1103 | <code>static void spec_claim_begin(Cg &#42;g) {</code>                                                                           |
| `spec_attempt_next_fence`        | function | 1111 | <code>static long spec_attempt_next_fence(Cg &#42;g) {</code>                                                                    |
| `spec_attempt_host`              | function | 1121 | <code>static const char &#42;spec_attempt_host(const char &#42;host, char buf&#91;256&#93;) {</code>                             |
| `spec_attempt_session`           | function | 1132 | <code>static const char &#42;spec_attempt_session(const char &#42;session) {</code>                                              |
| `spec_attempt_begin`             | function | 1141 | <code>static int spec_attempt_begin(Cg &#42;g, const char &#42;tag, const char &#42;agent,</code>                                |
| `spec_attempt_set_branch`        | function | 1187 | <code>int spec_attempt_set_branch(Cg &#42;g, const char &#42;tag, const char &#42;branch,</code>                                 |
| `spec_attempt_heartbeat`         | function | 1206 | <code>static int spec_attempt_heartbeat(Cg &#42;g, const char &#42;tag, const char &#42;agent,</code>                            |
| `spec_attempt_finish`            | function | 1247 | <code>static void spec_attempt_finish(Cg &#42;g, const char &#42;tag, const char &#42;state,</code>                              |
| `spec_attempt_owned`             | function | 1259 | <code>static bool spec_attempt_owned(Cg &#42;g, const char &#42;tag, const char &#42;agent,</code>                               |
| `spec_release_lease`             | function | 1276 | <code>static void spec_release_lease(Spec &#42;s, const char &#42;id);</code>                                                    |
| `spec_queue_release`             | function | 1282 | <code>static int spec_queue_release(Cg &#42;g, const char &#42;tag, const char &#42;agent,</code>                                |
| `spec_require_owner`             | function | 1309 | <code>static int spec_require_owner(Spec &#42;s, const char &#42;id, const char &#42;agent,</code>                               |
| `spec_set_status_owned`          | function | 1342 | <code>static int spec_set_status_owned(Spec &#42;s, const char &#42;id, const char &#42;status,</code>                           |
| `spec_agent_leased`              | function | 1399 | <code>static const char &#42;spec_agent_leased(const Spec &#42;s, const char &#42;agent) {</code>                                |
| `spec_current_for_agent`         | function | 1430 | <code>static const char &#42;spec_current_for_agent(const Spec &#42;s, const char &#42;agent) {</code>                           |
| `spec_stale_tasks`               | function | 1440 | <code>static int spec_stale_tasks(const Spec &#42;s, const char &#42;&#42;&#42;out) {</code>                                     |
| `join_list`                      | function | 1479 | <code>static char &#42;join_list(const Kvx &#42;k, const char &#42;sec, const char &#42;key) {</code>                            |
| `spec_note_outcome`              | function | 1494 | <code>static void spec_note_outcome(Spec &#42;s, const char &#42;id, const char &#42;body) {</code>                              |
| `spec_task_memories`             | function | 1530 | <code>static int spec_task_memories(Spec &#42;s, const char &#42;id, Memory &#42;&#42;out) {</code>                              |
| `spec_print_memories`            | function | 1559 | <code>static void spec_print_memories(Spec &#42;s, const char &#42;id) {</code>                                                  |
| `spec_live_leases`               | function | 1569 | <code>static int spec_live_leases(const Spec &#42;s, StrBuf &#42;b, bool json);</code>                                           |
| `spec_status_cmd`                | function | 1665 | <code>static int spec_status_cmd(Spec &#42;s, bool json) {</code>                                                                |
| `spec_mode_cmd`                  | function | 1809 | <code>static int spec_mode_cmd(Spec &#42;s, const char &#42;mode, bool json) {</code>                                            |
| `spec_next_cmd`                  | function | 1835 | <code>static int spec_next_cmd(Spec &#42;s, bool json) {</code>                                                                  |
| `spec_docs_verified`             | function | 1907 | <code>static bool spec_docs_verified(const Spec &#42;s) {</code>                                                                 |
| `spec_docs_finish`               | function | 1918 | <code>int spec_docs_finish(Cg &#42;cg, const char &#42;feature) {</code>                                                         |
| `spec_docs_cmd`                  | function | 1929 | <code>static int spec_docs_cmd(Spec &#42;s, const char &#42;action, const char &#42;agent,</code>                                |
| `spec_globs_overlap`             | function | 2074 | <code>bool spec_globs_overlap(const char &#42;a, const char &#42;b) {</code>                                                     |
| `spec_touches_conflict`          | function | 2089 | <code>static bool spec_touches_conflict(Spec &#42;s, const char &#42;id, char &#42;other,</code>                                 |
| `spec_live_leases`               | function | 2171 | <code>static int spec_live_leases(const Spec &#42;s, StrBuf &#42;b, bool json) {</code>                                          |
| `spec_wave_cmd`                  | function | 2242 | <code>static int spec_wave_cmd(Spec &#42;s, bool json) {</code>                                                                  |
| `spec_lease_ahead_column`        | function | 2302 | <code>static bool spec_lease_ahead_column(Cg &#42;g) {</code>                                                                    |
| `spec_lease_ahead_set`           | function | 2315 | <code>static int spec_lease_ahead_set(Cg &#42;g, const char &#42;tag, const char &#42;reqs) {</code>                             |
| `spec_start_ahead`               | function | 2325 | <code>int spec_start_ahead(Cg &#42;g, const char &#42;tag, const char &#42;agent,</code>                                         |
| `spec_lease_upsert`              | function | 2346 | <code>static int spec_lease_upsert(Cg &#42;g, const char &#42;tag, const char &#42;agent,</code>                                 |
| `spec_release_lease`             | function | 2374 | <code>static void spec_release_lease(Spec &#42;s, const char &#42;id) {</code>                                                   |
| `spec_heartbeat_cmd`             | function | 2394 | <code>static int spec_heartbeat_cmd(Spec &#42;s, const char &#42;id, const char &#42;agent,</code>                               |
| `spec_reconcile_cmd`             | function | 2495 | <code>static int spec_reconcile_cmd(Spec &#42;s, bool repair, bool json) {</code>                                                |
| `spec_lease_ahead_set`           | function | 2543 | <code>static int spec_lease_ahead_set(Cg &#42;g, const char &#42;tag, const char &#42;reqs);</code>                              |
| `spec_claim_take`                | function | 2545 | <code>static int spec_claim_take(Cg &#42;g, Spec &#42;s, const char &#42;id, const char &#42;agent,</code>                       |
| `spec_claim_ex`                  | function | 2633 | <code>int spec_claim_ex(Cg &#42;g, const char &#42;root, const char &#42;feature,</code>                                         |
| `spec_claim`                     | function | 2646 | <code>int spec_claim(Cg &#42;g, const char &#42;root, const char &#42;feature, const char &#42;id,</code>                        |
| `spec_claim_cmd`                 | function | 2657 | <code>static int spec_claim_cmd(Spec &#42;s, const char &#42;id, const char &#42;agent,</code>                                   |
| `spec_ready_cmd`                 | function | 2863 | <code>static int spec_ready_cmd(Spec &#42;s, bool json) {</code>                                                                 |
| `spec_claim_next_cmd`            | function | 2973 | <code>static int spec_claim_next_cmd(Spec &#42;s, const char &#42;agent, const char &#42;host,</code>                            |
| `spec_start_cmd`                 | function | 3149 | <code>static int spec_start_cmd(Spec &#42;s, const char &#42;id, bool force, bool ahead,</code>                                  |
| `spec_verify_task`               | function | 3283 | <code>static int spec_verify_task(Spec &#42;s, const char &#42;id);</code>                                                       |
| `spec_implemented_cmd`           | function | 3285 | <code>static int spec_implemented_cmd(Spec &#42;s, const char &#42;id, const char &#42;agent,</code>                             |
| `spec_triage_distributed`        | function | 3380 | <code>static bool spec_triage_distributed(const char &#42;id, int code,</code>                                                   |
| `spec_run_verify`                | function | 3418 | <code>static int spec_run_verify(const char &#42;root, const char &#42;cmd, char &#42;tail,</code>                               |
| `spec_done_cmd`                  | function | 3448 | <code>static int spec_done_cmd(Spec &#42;s, const char &#42;id, const char &#42;agent,</code>                                    |
| `spec_journal_apply`             | function | 3655 | <code>int spec_journal_apply(Cg &#42;g, const char &#42;op, const char &#42;args, char &#42;err,</code>                          |
| `spec_graph_open`                | function | 3749 | <code>static bool spec_graph_open(Cg &#42;g, bool sync) {</code>                                                                 |
| `graph_symbol_row`               | function | 3756 | <code>static void graph_symbol_row(sqlite3_stmt &#42;st, char &#42;path, size_t pcap,</code>                                     |
| `path_has_qualifier`             | function | 3767 | <code>static bool path_has_qualifier(const char &#42;path, const char &#42;segment,</code>                                       |
| `qualified_path_score`           | function | 3790 | <code>static int qualified_path_score(const char &#42;qualified, const char &#42;path) {</code>                                  |
| `graph_symbol_named`             | function | 3824 | <code>static int graph_symbol_named(Cg &#42;g, const char &#42;lookup,</code>                                                    |
| `graph_symbol`                   | function | 3903 | <code>static int graph_symbol(Cg &#42;g, const char &#42;name, char &#42;&#42;touches, int ntouch,</code>                        |
| `task_tag`                       | function | 3939 | <code>static void task_tag(const Spec &#42;s, const char &#42;id, char &#42;out, size_t cap) {</code>                            |
| `pattern_hit`                    | function | 3949 | <code>static bool pattern_hit(const char &#42;pat, char &#42;&#42;paths, int np) {</code>                                        |
| `spec_verify_task`               | function | 3969 | <code>static int spec_verify_task(Spec &#42;s, const char &#42;id) {</code>                                                      |
| `trace_task`                     | function | 4032 | <code>static void trace_task(Spec &#42;s, Cg &#42;g, bool have_graph, const char &#42;id,</code>                                 |
| `spec_trace_cmd`                 | function | 4161 | <code>static int spec_trace_cmd(Spec &#42;s, const char &#42;id, bool sync, bool json) {</code>                                  |
| `spec_new_cmd`                   | function | 4333 | <code>static int spec_new_cmd(const char &#42;root_ov, const char &#42;feature, bool json) {</code>                              |
| `list_literal`                   | function | 4405 | <code>static char &#42;list_literal(const char &#42;csv) {</code>                                                                |
| `TaskSpec`                       | typedef  | 3809 | <code>typedef struct {</code>                                                                                                    |
| `spec_add_cmd`                   | function | 4437 | <code>static int spec_add_cmd(Spec &#42;s, const char &#42;id, const TaskSpec &#42;t,</code>                                     |
| `Lint`                           | typedef  | 4514 | <code>typedef struct { StrBuf b; int errors, warnings; } Lint;</code>                                                            |
| `lint_say`                       | function | 4516 | <code>static void lint_say(Lint &#42;l, bool error, const char &#42;id, const char &#42;fmt, ...) {</code>                       |
| `lint_cycle`                     | function | 4596 | <code>static void lint_cycle(Spec &#42;s, const char &#42;id, LintCyc &#42;c, Lint &#42;l) {</code>                              |
| `spec_lint_cmd`                  | function | 4677 | <code>static int spec_lint_cmd(Spec &#42;s, bool json) {</code>                                                                  |
| `spec_active_touches`            | function | 4818 | <code>int spec_active_touches(char &#42;&#42;&#42;out) {</code>                                                                  |
| `spec_active_tag`                | function | 4831 | <code>char &#42;spec_active_tag(void) {</code>                                                                                   |
| `spec_task_tag`                  | function | 4851 | <code>char &#42;spec_task_tag(const char &#42;requested) {</code>                                                                |
| `spec_resolve_task`              | function | 4886 | <code>char &#42;spec_resolve_task(const char &#42;requested, const char &#42;agent) {</code>                                     |
| `spec_load_tag`                  | function | 4922 | <code>static int spec_load_tag(Spec &#42;s, const char &#42;requested, const char &#42;&#42;id_out) {</code>                     |
| `spec_task_packet`               | function | 5106 | <code>char &#42;spec_task_packet(const char &#42;requested) {</code>                                                             |
| `spec_task_memories_tag`         | function | 5125 | <code>int spec_task_memories_tag(const char &#42;requested, Memory &#42;&#42;out) {</code>                                       |
| `cmd_spec`                       | function | 5141 | <code>int cmd_spec(int argc, char &#42;&#42;argv, bool json) {</code>                                                            |
