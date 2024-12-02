package database

import (
    "context"
    "fmt"
    "google.golang.org/grpc"
    pb "github.com/feichai0017/document-processor/proto/vanna"  // 需要生成 protobuf
    "github.com/feichai0017/document-processor/pkg/logger"
)

type SqlGenerator struct {
    logger     logger.Logger
    vannaConn  *grpc.ClientConn
    vannaClient pb.VannaServiceClient
}

type Config struct {
    APIKey      string
    Model       string
    DBType      string
    TrainData   []string
    GrpcAddress string  // Vanna gRPC 服务地址
}

func NewSqlGenerator(logger logger.Logger, cfg *Config) (*SqlGenerator, error) {
    // 建立 gRPC 连接
    conn, err := grpc.Dial(cfg.GrpcAddress, grpc.WithInsecure())
    if err != nil {
        return nil, fmt.Errorf("failed to connect to vanna service: %w", err)
    }

    client := pb.NewVannaServiceClient(conn)
    
    return &SqlGenerator{
        logger:      logger,
        vannaConn:   conn,
        vannaClient: client,
    }, nil
}

// convertMapToProto 将 map[string]interface{} 转换为 map[string]string
func convertMapToProto(m map[string]interface{}) map[string]string {
    result := make(map[string]string)
    for k, v := range m {
        result[k] = fmt.Sprintf("%v", v)
    }
    return result
}

// GenerateQuery 生成SQL查询
func (g *SqlGenerator) GenerateQuery(ctx context.Context, description string, dbContext map[string]interface{}) (string, error) {
    g.logger.Info("Generating SQL query",
        logger.String("description", description),
        logger.Any("context", dbContext),
    )

    req := &pb.GenerateSQLRequest{
        Question: description,
        Context:  convertMapToProto(dbContext),
    }

    resp, err := g.vannaClient.GenerateSQL(ctx, req)
    if err != nil {
        return "", fmt.Errorf("failed to generate SQL: %w", err)
    }

    g.logger.Info("SQL query generated",
        logger.String("query", resp.Sql),
    )

    return resp.Sql, nil
}

// TrainModel 训练模型
func (g *SqlGenerator) TrainModel(ctx context.Context, trainData []string) error {
    g.logger.Info("Training model with sample queries",
        logger.Int("sampleCount", len(trainData)),
    )

    resp, err := g.vannaClient.Train(ctx, &pb.TrainRequest{
        Data: trainData,
    })
    if err != nil {
        return fmt.Errorf("failed to train model: %w", err)
    }

    if !resp.Success {
        return fmt.Errorf("training failed: %s", resp.Message)
    }

    g.logger.Info("Model training completed")
    return nil
}

// ValidateQuery 验证生成的SQL
func (g *SqlGenerator) ValidateQuery(ctx context.Context, query string) error {
    g.logger.Info("Validating SQL query",
        logger.String("query", query),
    )

    // 使用 Vanna 验证 SQL
    valid, err := g.vannaClient.ValidateSQL(ctx, &pb.ValidateSQLRequest{
        Sql: query,
    })
    if err != nil {
        return fmt.Errorf("failed to validate SQL: %w", err)
    }

    if !valid.IsValid {
        return fmt.Errorf("invalid SQL query: %s", valid.Message)
    }

    return nil
}

// ExplainQuery 解释SQL查询
func (g *SqlGenerator) ExplainQuery(ctx context.Context, query string) (string, error) {
    g.logger.Info("Explaining SQL query",
        logger.String("query", query),
    )

    // 使用 Vanna 解释 SQL
    explanation, err := g.vannaClient.ExplainSQL(ctx, &pb.ExplainSQLRequest{
        Sql: query,
    })
    if err != nil {
        return "", fmt.Errorf("failed to explain SQL: %w", err)
    }

    return explanation.Explanation, nil
}

func (g *SqlGenerator) Close() error {
    if g.vannaConn != nil {
        return g.vannaConn.Close()
    }
    return nil
}