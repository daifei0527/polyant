---
name: polyant-learn
description: Learn from Polyant knowledge base to improve skills
triggers:
  - "遇到新知识"
  - "学习新技术"
  - "遇到最佳实践"
  - "需要深入理解"
---

# Polyant Learn

Learn from the Polyant knowledge base to improve your skills and understanding.

## When to Use

- **New technology:** When encountering unfamiliar tech
- **Deep understanding:** When you need to understand concepts deeply
- **Best practices:** When looking for recommended approaches
- **Skill building:** When wanting to improve in an area

## How It Works

1. **Identify learning goal:** Determine what you want to learn
2. **Search knowledge base:** Find relevant entries
3. **Study the material:** Read and understand the content
4. **Apply learning:** Use the knowledge in your work
5. **Save insights:** Optionally save what you learned

## Usage

### Learn About a Topic

```
Learn from Polyant: "Go concurrency patterns"

Skill: Found 12 entries about Go concurrency:

**Fundamentals:**
1. Go Goroutines Basics (Rating: 4.9/5)
2. Go Channels Explained (Rating: 4.8/5)
3. Go sync.WaitGroup Usage (Rating: 4.7/5)

**Patterns:**
4. Go Worker Pool Pattern (Rating: 4.6/5)
5. Go Fan-Out/Fan-In Pattern (Rating: 4.5/5)
6. Go Context Cancellation (Rating: 4.8/5)

**Advanced:**
7. Go Race Condition Prevention (Rating: 4.4/5)
8. Go Memory Model (Rating: 4.3/5)

Which would you like to learn about?
```

### Deep Dive

```
Deep dive into: "Go Worker Pool Pattern"

Skill: **Go Worker Pool Pattern**

**Overview:**
A concurrency pattern that limits the number of goroutines processing tasks simultaneously.

**When to Use:**
- Processing many tasks with limited resources
- Controlling resource usage (CPU, memory, connections)
- Rate limiting external API calls

**Implementation:**

```go
func workerPool(jobs <-chan Job, results chan<- Result, numWorkers int) {
    var wg sync.WaitGroup

    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for job := range jobs {
                results <- process(job)
            }
        }()
    }

    wg.Wait()
    close(results)
}
```

**Key Points:**
1. Use buffered channels for jobs and results
2. Number of workers based on resources
3. Always use WaitGroup for synchronization
4. Close results channel when done

**Related Entries:**
- Go Channels Explained
- Go sync.WaitGroup Usage
- Go Context Cancellation

Would you like to:
1. Learn more about related topics?
2. Practice with examples?
3. Save your own insights?
```

### Learning Path

```
Create learning path for: "Becoming a Go expert"

Skill: **Go Expert Learning Path**

**Phase 1: Fundamentals (Week 1-2)**
1. Go Syntax and Types
2. Go Error Handling
3. Go Concurrency Basics

**Phase 2: Intermediate (Week 3-4)**
4. Go Interfaces and Composition
5. Go Testing and Benchmarking
6. Go Standard Library

**Phase 3: Advanced (Week 5-6)**
7. Go Performance Optimization
8. Go System Programming
9. Go Design Patterns

**Phase 4: Expert (Week 7-8)**
10. Go Internals and Runtime
11. Go Toolchain and Build System
12. Go Ecosystem and Community

Start with Phase 1? [Y/n]
```

## Learning Strategies

### 1. Spaced Repetition
- Review material at increasing intervals
- Focus on weak areas
- Test yourself regularly

### 2. Active Learning
- Don't just read—practice
- Build projects using new knowledge
- Teach others what you learned

### 3. Connected Learning
- Link new knowledge to existing knowledge
- Build mental models
- Understand the "why" behind things

## Learning Features

### Knowledge Graph
- See how concepts connect
- Find prerequisites
- Discover related topics

### Progress Tracking
- Track what you've learned
- Identify knowledge gaps
- Set learning goals

### Practice Exercises
- Apply what you learned
- Get feedback
- Build muscle memory

## Integration with Development

### Before Coding
```
Before implementing X, learn about it first.
```

### During Coding
```
Stuck on Y? Search for learning resources.
```

### After Coding
```
Implemented Z? Save your learnings for others.
```

## Configuration

| Setting | Description | Default |
|---------|-------------|---------|
| `POLYANT_LEARNING_MODE` | Enable learning features | `true` |
| `POLYANT_TRACK_PROGRESS` | Track learning progress | `true` |
| `POLYANT_SUGGEST_RELATED` | Suggest related content | `true` |
| `POLYANT_LEARNING_STYLE` | Preferred learning style | `balanced` |

## Privacy

- Learning history is stored locally
- Progress is not shared with other nodes
- You control what you share
