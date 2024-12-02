package models



// DBT配置
type DbtConfig struct {
    ProjectName     string            `yaml:"name" json:"name"`
    Version         string            `yaml:"version" json:"version"`
    Profile         string            `yaml:"profile" json:"profile"`
    ConfigVersion   int               `yaml:"config-version" json:"configVersion"`
    ModelPaths      []string          `yaml:"model-paths" json:"modelPaths"`
    SourcePaths     []string          `yaml:"source-paths" json:"sourcePaths"`
    TestPaths       []string          `yaml:"test-paths" json:"testPaths"`
    AnalysisPaths   []string          `yaml:"analysis-paths" json:"analysisPaths"`
    MacroPaths      []string          `yaml:"macro-paths" json:"macroPaths"`
    Target          string            `yaml:"target" json:"target"`
    CleanTargets    []string          `yaml:"clean-targets" json:"cleanTargets"`
    RequiredVars    []string          `yaml:"require-vars" json:"requireVars,omitempty"`
    Models          map[string]interface{} `yaml:"models" json:"models"`
}

// 数据库配置
type DatabaseConfig struct {
    Type        string `yaml:"type" json:"type"`
    Host        string `yaml:"host" json:"host"`
    Port        int    `yaml:"port" json:"port"`
    User        string `yaml:"user" json:"user"`
    Password    string `yaml:"password" json:"password,omitempty"`
    Database    string `yaml:"database" json:"database"`
    Schema      string `yaml:"schema" json:"schema"`
    ThreadCount int    `yaml:"threads" json:"threads"`
}

// DBT模型定义
type DbtModel struct {
    Name           string                 `yaml:"name" json:"name"`
    Description    string                 `yaml:"description" json:"description"`
    MaterializedAs string                 `yaml:"materialized" json:"materializedAs"`
    Schema         string                 `yaml:"schema" json:"schema"`
    Database      string                 `yaml:"database" json:"database,omitempty"`
    Columns       []ModelColumn          `yaml:"columns" json:"columns"`
    Tests         []string               `yaml:"tests,omitempty" json:"tests,omitempty"`
    Tags          []string               `yaml:"tags,omitempty" json:"tags,omitempty"`
    Config        map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
    Dependencies  []string               `yaml:"depends_on,omitempty" json:"dependsOn,omitempty"`
    Meta          map[string]interface{} `yaml:"meta,omitempty" json:"meta,omitempty"`
}

// 列定义
type ModelColumn struct {
    Name        string   `yaml:"name" json:"name"`
    Description string   `yaml:"description" json:"description"`
    Type        string   `yaml:"type" json:"type"`
    Tests       []string `yaml:"tests,omitempty" json:"tests,omitempty"`
    Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// DBT源定义
type DbtSource struct {
    Name        string       `yaml:"name" json:"name"`
    Description string       `yaml:"description" json:"description"`
    Database    string       `yaml:"database" json:"database"`
    Schema      string       `yaml:"schema" json:"schema"`
    Tables      []SourceTable `yaml:"tables" json:"tables"`
}

// 源表定义
type SourceTable struct {
    Name        string       `yaml:"name" json:"name"`
    Description string       `yaml:"description" json:"description"`
    Columns     []ModelColumn `yaml:"columns" json:"columns"`
    Tests       []string     `yaml:"tests,omitempty" json:"tests,omitempty"`
    LoadedAt    string       `yaml:"loaded_at_field,omitempty" json:"loadedAtField,omitempty"`
}