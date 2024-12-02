// pkg/queue/queue.go
package queue

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/hibiken/asynq"
    "github.com/redis/go-redis/v9"
)

// TaskType 定义任务类型
const (
    TaskTypeDocumentProcess = "document:process"
    TaskTypeImageProcess   = "image:process"
    TaskTypePDFProcess    = "pdf:process"
    TaskTypeWordProcess   = "word:process"
)

// Queue 接口定义
type Queue interface {
    Enqueue(ctx context.Context, task *Task) error
    GetTaskStatus(ctx context.Context, taskID string) (*TaskStatus, error)
    CancelTask(ctx context.Context, taskID string) error
    SaveFinalStatus(ctx context.Context, status *TaskStatus) error
}

// Task 定义任务结构
type Task struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    Priority  int                    `json:"priority"`
    Payload   map[string]interface{} `json:"payload"`
    Metadata  map[string]string      `json:"metadata"`
    CreatedAt time.Time             `json:"createdAt"`
}

// TaskStatus 定义任务状态
type TaskStatus struct {
    TaskID     string    `json:"taskId"`
    Status     string    `json:"status"`
    Progress   float64   `json:"progress"`
    Error      string    `json:"error,omitempty"`
    StartedAt  time.Time `json:"startedAt"`
    FinishedAt time.Time `json:"finishedAt,omitempty"`
}

// AsynqQueue 实现
type AsynqQueue struct {
    client    *asynq.Client
    inspector *asynq.Inspector
    server    *asynq.Server
    redis     *redis.Client
}

// QueueConfig 定义队列配置
type QueueConfig struct {
    RedisAddr      string
    RedisDB        int
    MaxRetries     int
    RetryDelay     time.Duration
    ProcessTimeout time.Duration
    Concurrency    int
}

// GetQueue 获取队列实例
func GetQueue() (*AsynqQueue, error) {
    asynqQueue, err := NewAsynqQueue(&QueueConfig{
        RedisAddr:      "localhost:6379",
        RedisDB:        0,
        MaxRetries:     3,
        RetryDelay:     1 * time.Minute,
        ProcessTimeout: 30 * time.Minute,
        Concurrency:    5,
    })
    if err != nil {
        return nil, err
    }
    return asynqQueue, nil
}

// NewAsynqQueue 创建新的队列实例
func NewAsynqQueue(cfg *QueueConfig) (*AsynqQueue, error) {
    redisOpt := asynq.RedisClientOpt{
        Addr: cfg.RedisAddr,
        DB:   cfg.RedisDB,
    }

    // 创建 Redis 客户端
    redisClient := redis.NewClient(&redis.Options{
        Addr: cfg.RedisAddr,
        DB:   cfg.RedisDB,
    })

    // 创建客户端
    client := asynq.NewClient(redisOpt)

    // 创建检查器
    inspector := asynq.NewInspector(redisOpt)

    // 创建服务器
    serverOpt := asynq.Config{
        Concurrency: cfg.Concurrency,
        Queues: map[string]int{
            "critical": 6,
            "default": 3,
            "low":     1,
        },
        RetryDelayFunc: func(n int, err error, task *asynq.Task) time.Duration {
            return cfg.RetryDelay
        },
    }
    server := asynq.NewServer(redisOpt, serverOpt)

    return &AsynqQueue{
        client:    client,
        inspector: inspector,
        server:    server,
        redis:     redisClient,
    }, nil
}

// Enqueue 将任务加入队列
func (q *AsynqQueue) Enqueue(ctx context.Context, task *Task) error {
    // 序列化整个任务
    payload, err := json.Marshal(task)
    if err != nil {
        return fmt.Errorf("failed to marshal task: %w", err)
    }

    // 设置任务选项
    opts := []asynq.Option{
        asynq.ProcessIn(time.Second),
        asynq.MaxRetry(3),
        asynq.Timeout(30 * time.Minute),
        asynq.TaskID(task.ID),
    }

    // 根据优先选择队列
    switch task.Priority {
    case 1:
        opts = append(opts, asynq.Queue("critical"))
    case 2:
        opts = append(opts, asynq.Queue("default"))
    default:
        opts = append(opts, asynq.Queue("low"))
    }

    // 创建并入队任务
    t := asynq.NewTask(task.Type, payload, opts...)
    info, err := q.client.EnqueueContext(ctx, t)
    if err != nil {
        return fmt.Errorf("failed to enqueue task: %w", err)
    }

    // 记录任务ID
    task.ID = info.ID

    return nil
}

// GetTaskStatus 获取任务状态
func (q *AsynqQueue) GetTaskStatus(ctx context.Context, taskID string) (*TaskStatus, error) {
    // 首先尝试从 Redis 获取状态
    key := fmt.Sprintf("task_status:%s", taskID)
    data, err := q.redis.Get(ctx, key).Bytes()
    if err != nil && err != redis.Nil {
        return nil, fmt.Errorf("failed to get status from redis: %w", err)
    }

    if err == nil {
        // 如果找到了保存的状态，直接返回
        var status TaskStatus
        if err := json.Unmarshal(data, &status); err != nil {
            return nil, fmt.Errorf("failed to unmarshal status: %w", err)
        }
        return &status, nil
    }

    // 如果 Redis 中没有，从所有队列中查找
    queues := []string{"critical", "default", "low"}
    var info *asynq.TaskInfo
    var lastErr error

    for _, queueName := range queues {
        info, err = q.inspector.GetTaskInfo(queueName, taskID)
        if err == nil {
            break
        }
        lastErr = err
    }

    if lastErr != nil {
        return nil, fmt.Errorf("task not found in any queue: %w", lastErr)
    }

    status := convertAsynqStatus(info)
    
    // 保存状态到 Redis
    if err := q.SaveFinalStatus(ctx, status); err != nil {
        fmt.Printf("Failed to save status for task %s: %v\n", taskID, err)
    }

    return status, nil
}

// CancelTask 取消任务
func (q *AsynqQueue) CancelTask(ctx context.Context, taskID string) error {
    // 尝试在所有队列中取消任务
    queues := []string{"critical", "default", "low"}
    var lastErr error
    
    for _, queue := range queues {
        err := q.inspector.DeleteTask(queue, taskID)
        if err == nil {
            return nil
        }
        lastErr = err
    }

    return fmt.Errorf("failed to cancel task: %w", lastErr)
}

// SaveFinalStatus 保存最终任务状态
func (q *AsynqQueue) SaveFinalStatus(ctx context.Context, status *TaskStatus) error {
    // 使用 Redis 客户端保存状态
    key := fmt.Sprintf("task_status:%s", status.TaskID)
    data, err := json.Marshal(status)
    if err != nil {
        return fmt.Errorf("failed to marshal status: %w", err)
    }
    
    // 设置过期时间（例如 24 小时）
    err = q.redis.Set(ctx, key, data, 24*time.Hour).Err()
    if err != nil {
        return fmt.Errorf("failed to save status: %w", err)
    }
    
    return nil
}

// convertAsynqStatus 将 asynq 状态转换为 TaskStatus
func convertAsynqStatus(info *asynq.TaskInfo) *TaskStatus {
    status := &TaskStatus{
        TaskID:    info.ID,
        StartedAt: info.NextProcessAt,
        FinishedAt: time.Now(),
    }

    switch info.State {
    case asynq.TaskStatePending:
        status.Status = "pending"
    case asynq.TaskStateActive:
        status.Status = "running"
        status.Progress = 0.5
    case asynq.TaskStateCompleted:
        status.Status = "completed"
        status.Progress = 1.0
        status.FinishedAt = info.CompletedAt
    case asynq.TaskStateRetry:
        status.Status = "failed"
        status.Error = info.LastErr
    }

    return status
}