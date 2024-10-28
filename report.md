# Game of Life Report 

**Team members**: Dylan Lin *(nj23727)*, Tommy Dong *(of23238)*, Zhitong Liu *(wr23070)*

---

### Parallel Implementation report, more details
并行实施报告，更多详细信息

- Discuss the goroutines you used and how they work together: this is not just your workers! Refer to step 5 of the task description as a starting point

讨论你使用的 goroutines 以及它们如何协同工作：这不仅仅是你的 worker！请参阅任务描述的步骤 5 作为起点

- Explain and analyse the benchmark results obtained. Prescriptive guidance on obtaining benchmarks is described in Question 1d for concurrency lab 1.. Look at the code provided to obtain the graph produced. To adapt this to your Gol implementation consider basing your code around func TestGol provided in the skeleton code for Gol. Remember for the benchmark we are concerned with performance not correctness.

解释和分析获得的基准测试结果。有关获取基准测试的说明性指南在并发实验室 1 的问题 1d 中描述。查看提供的代码以获取生成的图形。要使其适应您的 Gol 实现，请考虑将代码基于 Gol 框架代码中提供的 func TestGol。请记住，对于基准测试，我们关心的是性能，而不是正确性。

Also refer to Google's documentation for benchmarking.
另请参阅 Google 的文档以进行基准测试。
You may want to consider using graphs to visualise your benchmarks. To obtain your graph refer to Question 1d for concurrency lab 1. Remember you do not have to use Python to plot the graph you can use Excel, MATLAB, Libre Office etc...
您可能需要考虑使用图表来可视化您的基准。要获取图表，请参阅并发实验 1 的问题 1d。请记住，您不必使用 Python 来绘制图形，您可以使用 Excel、MATLAB、Libre Office 等......

Analyse how your implementation scales as more workers are added. Remember you have been given a template solution for this in the solution for concurrency lab 1. Adapt the text in README.md in the zip file.
分析您的实施如何随着更多工作线程的添加而扩展。请记住，在并发实验 1 的解决方案中，您已获得此解决方案的模板解决方案。在 zip 文件中调整 README.md 中的文本。

Briefly discuss your methodology for acquiring any results or measurements. This will relate directly to how your benchmark code is parameterised
简要讨论您获取任何结果或测量值的方法。这将直接关系到基准测试代码的参数化方式

### A little more advanced  更高级一点
Consider implementing and benchmarking differently parameterised and differently designed implementations. For example, a pure channels implementation which does not use shared memory.
考虑实现不同参数化和不同设计的实现并对其进行基准测试。例如，不使用共享内存的纯通道实现。

To go a little deeper, look at question 1g which involves use of the powerful tool, pprof and question 1i.
要更深入地了解，请查看问题 1g，其中涉及使用强大的工具 pprof 和问题 1i。

Using these tools for Gol will add extra depth to your report.
使用 Gol 的这些工具将为您的报告增加额外的深度。

Only a few groups did this last year so do not worry if you do not complete this part
去年只有少数小组这样做了，所以如果您没有完成这部分，请不要担心

Distributed Implementation report, more details
分布式实施报告，更多详细信息

Discuss the system design and reasons for any decisions made. Consider using a diagram to aid your discussion. Once again refer to the diagrams provided in the task description as a starting point.
讨论系统设计和做出任何决定的原因。考虑使用图表来帮助讨论。再次参考任务描述中提供的图表作为起点。

Explain what data is sent over the network, when, and why it is necessary.
说明通过网络发送哪些数据、何时发送以及为什么需要发送数据。

Discuss how your system might scale with the addition of other distributed components.
讨论您的系统如何通过添加其他分布式组件进行扩展。

Briefly discuss your methodology for acquiring any results or measurements.
简要讨论您获取任何结果或测量值的方法。

Note that our expectations with respect to empirical tests and benchmarking are far lower for the distributed component on the coursework and a single graph, showing how performance scales with the number of worker nodes will normally be ample.
请注意，我们对经验测试和基准测试的期望要低得多，因为课程作业和单个图表上的分布式组件显示了性能如何随着 worker 节点的数量而扩展通常足够。

Identify how components of your system disappearing (e.g., broken network connections) might affect the overall system and its results.
确定系统组件消失（例如，网络连接中断）如何影响整个系统及其结果。