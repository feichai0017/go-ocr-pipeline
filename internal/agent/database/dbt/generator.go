package dbt

import (
    "bytes"
    "fmt"
    "os"
    "path/filepath"
    "text/template"
    "time"
    "gopkg.in/yaml.v3"
    "github.com/feichai0017/document-processor/pkg/logger"
    "github.com/feichai0017/document-processor/internal/models"
)

// DBT生成器
type Generator struct {
    config *models.DbtConfig
    logger logger.Logger
}

// 创建新的生成器
func NewGenerator(config *models.DbtConfig, logger logger.Logger) *Generator {
    return &Generator{
        config: config,
        logger: logger,
    }
}

// 生成DBT项目结构
func (g *Generator) GenerateProjectStructure(outputPath string) error {
    // 创建目录结构
    directories := []string{
        "models",
        "models/staging",
        "models/marts",
        "tests",
        "macros",
        "seeds",
        "analyses",
    }

    for _, dir := range directories {
        if err := os.MkdirAll(filepath.Join(outputPath, dir), 0755); err != nil {
            return fmt.Errorf("failed to create directory %s: %w", dir, err)
        }
    }

    // 生成项目配置文件
    if err := g.generateProjectConfig(outputPath); err != nil {
        return err
    }

    return nil
}

// 生成DBT项目配置
func (g *Generator) generateProjectConfig(outputPath string) error {
    configPath := filepath.Join(outputPath, "dbt_project.yml")
    
    data, err := yaml.Marshal(g.config)
    if err != nil {
        return fmt.Errorf("failed to marshal config: %w", err)
    }

    if err := os.WriteFile(configPath, data, 0644); err != nil {
        return fmt.Errorf("failed to write config file: %w", err)
    }

    return nil
}

// 从CSV schema生成模型
func (g *Generator) GenerateModelsFromCSV(csvSchema map[string]string, targetDB string) (*models.DbtModel, error) {
    // 创建基础模型
    model := &models.DbtModel{
        Name:           fmt.Sprintf("stg_%s", g.config.ProjectName),
        Description:    "Generated model from CSV data",
        MaterializedAs: "table",
        Schema:        "staging",
        Columns:       make([]models.ModelColumn, 0),
        Tests:         []string{"unique", "not_null"},
        Tags:          []string{"staging", "generated"},
    }

    // 添加列
    for name, dataType := range csvSchema {
        column := models.ModelColumn{
            Name:        name,
            Description: fmt.Sprintf("Generated column from CSV field %s", name),
            Type:        g.mapDataType(dataType, targetDB),
            Tests:       []string{"not_null"},
        }
        model.Columns = append(model.Columns, column)
    }

    return model, nil
}

// 生成SQL转换
func (g *Generator) GenerateSQL(model *models.DbtModel) (string, error) {
    const sqlTemplate = `
{{- /*
    Generated SQL model for {{ .Name }}
    Created at: {{ now }}
*/ -}}

WITH source AS (
    SELECT * FROM {{ .Schema }}.raw_data
),

transformed AS (
    SELECT
        {{- range .Columns }}
        {{ .Name }} AS {{ .Name }},
        {{- end }}
        _loaded_at AS loaded_at
    FROM source
)

SELECT * FROM transformed
WHERE loaded_at > (SELECT MAX(loaded_at) FROM {{ .Schema }}.{{ .Name }})
`

    tmpl, err := template.New("sql").Funcs(template.FuncMap{
        "now": time.Now().UTC().Format,
    }).Parse(sqlTemplate)
    if err != nil {
        return "", fmt.Errorf("failed to parse template: %w", err)
    }

    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, model); err != nil {
        return "", fmt.Errorf("failed to execute template: %w", err)
    }

    return buf.String(), nil
}

// 生成模型YAML
func (g *Generator) GenerateModelYAML(model *models.DbtModel) (string, error) {
    yamlData := map[string]interface{}{
        "version": 2,
        "models": []interface{}{
            map[string]interface{}{
                "name":        model.Name,
                "description": model.Description,
                "columns":     model.Columns,
                "tests":       model.Tests,
                "tags":       model.Tags,
                "config": map[string]interface{}{
                    "materialized": model.MaterializedAs,
                    "schema":       model.Schema,
                },
            },
        },
    }

    data, err := yaml.Marshal(yamlData)
    if err != nil {
        return "", fmt.Errorf("failed to marshal model YAML: %w", err)
    }

    return string(data), nil
}

// 数据类型映射
func (g *Generator) mapDataType(sourceType, targetDB string) string {
    typeMapping := map[string]map[string]string{
        "mysql": {
            "string":  "VARCHAR(255)",
            "number":  "DECIMAL(18,2)",
            "integer": "INT",
            "float":   "FLOAT",
            "boolean": "BOOLEAN",
            "date":    "DATE",
            "timestamp": "TIMESTAMP",
        },
        "postgresql": {
            "string":  "TEXT",
            "number":  "NUMERIC(18,2)",
            "integer": "INTEGER",
            "float":   "DOUBLE PRECISION",
            "boolean": "BOOLEAN",
            "date":    "DATE",
            "timestamp": "TIMESTAMP",
        },
        "snowflake": {
            "string":  "VARCHAR",
            "number":  "NUMBER(18,2)",
            "integer": "INTEGER",
            "float":   "FLOAT",
            "boolean": "BOOLEAN",
            "date":    "DATE",
            "timestamp": "TIMESTAMP_NTZ",
        },
    }

    if mapping, ok := typeMapping[targetDB]; ok {
        if dbType, ok := mapping[sourceType]; ok {
            return dbType
        }
    }

    return "TEXT" // 默认类型
}

