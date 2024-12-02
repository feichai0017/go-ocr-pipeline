package dbt

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    
    "github.com/feichai0017/document-processor/pkg/logger"
    "github.com/feichai0017/document-processor/internal/models"
    "github.com/feichai0017/document-processor/internal/agent/database/dbt"
)

type Service struct {
    generator *dbt.Generator
    logger    logger.Logger
    config    *models.DbtConfig
    workDir   string
}

func NewService(config *models.DbtConfig, logger logger.Logger, workDir string) *Service {
    return &Service{
        generator: dbt.NewGenerator(config, logger),
        logger:    logger,
        config:    config,
        workDir:   workDir,
    }
}

// 初始化DBT项目
func (s *Service) InitializeProject(ctx context.Context) error {
    s.logger.Info("Initializing DBT project",
        logger.String("project", s.config.ProjectName),
        logger.String("workDir", s.workDir),
    )

    if err := s.generator.GenerateProjectStructure(s.workDir); err != nil {
        return fmt.Errorf("failed to generate project structure: %w", err)
    }

    // 运行dbt初始化命令
    if err := s.runDbtCommand(ctx, "init"); err != nil {
        return fmt.Errorf("failed to initialize dbt project: %w", err)
    }

    return nil
}

// 从CSV生成模型
func (s *Service) GenerateModelsFromCSV(
    ctx context.Context,
    csvSchema map[string]string,
    targetDB string,
) error {
    s.logger.Info("Generating DBT models from CSV schema",
        logger.String("targetDB", targetDB),
    )

    // 生成模型
    model, err := s.generator.GenerateModelsFromCSV(csvSchema, targetDB)
    if err != nil {
        return fmt.Errorf("failed to generate model: %w", err)
    }

    // 生成SQL
    sql, err := s.generator.GenerateSQL(model)
    if err != nil {
        return fmt.Errorf("failed to generate SQL: %w", err)
    }

    // 生成YAML
    yaml, err := s.generator.GenerateModelYAML(model)
    if err != nil {
        return fmt.Errorf("failed to generate YAML: %w", err)
    }

    // 写入文件
    modelDir := filepath.Join(s.workDir, "models", "staging")
    
    if err := os.WriteFile(
        filepath.Join(modelDir, fmt.Sprintf("%s.sql", model.Name)),
        []byte(sql),
        0644,
    ); err != nil {
        return fmt.Errorf("failed to write SQL file: %w", err)
    }

    if err := os.WriteFile(
        filepath.Join(modelDir, fmt.Sprintf("%s.yml", model.Name)),
        []byte(yaml),
        0644,
    ); err != nil {
        return fmt.Errorf("failed to write YAML file: %w", err)
    }

    return nil
}

// 运行DBT命令
func (s *Service) RunDbt(ctx context.Context, args ...string) error {
    return s.runDbtCommand(ctx, args...)
}

// 执行DBT命令
func (s *Service) runDbtCommand(ctx context.Context, args ...string) error {
    cmd := exec.CommandContext(ctx, "dbt", args...)
    cmd.Dir = s.workDir
    cmd.Env = append(os.Environ(),
        fmt.Sprintf("DBT_PROFILES_DIR=%s", s.workDir),
    )

    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("dbt command failed: %w\noutput: %s", err, string(output))
    }

    s.logger.Debug("DBT command executed successfully",
        logger.String("command", "dbt "+string(args[0])),
        logger.String("output", string(output)),
    )

    return nil
}