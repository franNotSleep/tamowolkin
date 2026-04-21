package queue

type TaskID string

type Task struct {
	ID          TaskID
	Title       string
	Description string
	BranchName  string
}

type Queue struct {
	jobs chan Task
}

func NewQueue() *Queue {
	return &Queue{
		jobs: make(chan Task),
	}
}

func (q *Queue) Enqueue(id, title, description, branchName string) {
	q.jobs <- Task{ID: TaskID(id), Title: title, Description: description, BranchName: branchName}
}

func (q *Queue) Dequeue() <-chan Task {
	return q.jobs
}
