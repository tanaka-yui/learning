package com.example;

import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.ThreadPoolExecutor;
import java.util.concurrent.TimeUnit;

public class ThreadPoolDemo {

    private static final DateTimeFormatter FORMATTER = DateTimeFormatter.ofPattern("HH:mm:ss.SSS");

    private static long fibonacci(int n) {
        if (n <= 1) return n;
        return fibonacci(n - 1) + fibonacci(n - 2);
    }

    private static int envInt(String name, int defaultValue) {
        String val = System.getenv(name);
        return (val != null && !val.isEmpty()) ? Integer.parseInt(val) : defaultValue;
    }

    public static void main(String[] args) {
        int poolSize = envInt("POOL_SIZE", 2);
        int fibN = envInt("FIBONACCI_N", 42);
        int producerIntervalMs = envInt("PRODUCER_INTERVAL_MS", 100);

        System.out.println("=== ThreadPool Demo ===");
        System.out.printf("POOL_SIZE=%d, FIBONACCI_N=%d, PRODUCER_INTERVAL_MS=%d%n", poolSize, fibN, producerIntervalMs);
        System.out.println();

        ConcurrentLinkedQueue<Integer> taskQueue = new ConcurrentLinkedQueue<>();

        ThreadPoolExecutor executor = new ThreadPoolExecutor(
                poolSize, poolSize, 0L, TimeUnit.MILLISECONDS, new LinkedBlockingQueue<>());

        // Shutdown hook for graceful termination
        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            System.out.println("\nShutting down...");
            executor.shutdownNow();
        }));

        // Thread 1: Monitor - prints thread pool state every 5 seconds
        Thread monitor = new Thread(() -> {
            while (!Thread.currentThread().isInterrupted()) {
                String now = LocalDateTime.now().format(FORMATTER);
                System.out.printf("[%s] [Monitor] poolSize=%d, activeCount=%d, queueSize=%d, completedTasks=%d, pendingQueue=%d%n",
                        now,
                        executor.getPoolSize(),
                        executor.getActiveCount(),
                        executor.getQueue().size(),
                        executor.getCompletedTaskCount(),
                        taskQueue.size());
                try {
                    Thread.sleep(5000);
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                }
            }
        }, "monitor");
        monitor.setDaemon(true);

        // Thread 2: Consumer - polls from taskQueue and submits to thread pool
        Thread consumer = new Thread(() -> {
            while (!Thread.currentThread().isInterrupted()) {
                Integer n = taskQueue.poll();
                if (n != null) {
                    executor.submit(() -> {
                        String threadName = Thread.currentThread().getName();
                        String startTime = LocalDateTime.now().format(FORMATTER);
                        System.out.printf("[%s] [Worker:%s] Start fibonacci(%d)%n", startTime, threadName, n);

                        long startMs = System.currentTimeMillis();
                        long result = fibonacci(n);
                        long durationMs = System.currentTimeMillis() - startMs;

                        String endTime = LocalDateTime.now().format(FORMATTER);
                        System.out.printf("[%s] [Worker:%s] Done fibonacci(%d)=%d (%dms)%n", endTime, threadName, n, result, durationMs);
                    });
                } else {
                    try {
                        Thread.sleep(100);
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                    }
                }
            }
        }, "consumer");
        consumer.setDaemon(true);

        // Thread 3: Producer - adds tasks to the queue at a configurable interval
        Thread producer = new Thread(() -> {
            int taskId = 0;
            while (!Thread.currentThread().isInterrupted()) {
                taskId++;
                taskQueue.add(fibN);
                String now = LocalDateTime.now().format(FORMATTER);
                System.out.printf("[%s] [Producer] Enqueued task #%d (fibonacci(%d))%n", now, taskId, fibN);
                try {
                    Thread.sleep(producerIntervalMs);
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                }
            }
        }, "producer");
        producer.setDaemon(true);

        monitor.start();
        consumer.start();
        producer.start();

        // Keep main thread alive
        try {
            Thread.currentThread().join();
        } catch (InterruptedException e) {
            // exit
        }
    }
}
