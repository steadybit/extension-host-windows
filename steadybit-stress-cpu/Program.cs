using System.CommandLine;
using System.Diagnostics;
using SteadybitCpuStress;

var rootCommand = new RootCommand("Steadybit CPU stress utility that is used by the Windows Host Extension to provide CPU based attacks.");

var durationOption = new Option<int>(name: "--duration", description: "Duration of the CPU stress execution in seconds.", getDefaultValue: () => 30);
durationOption.AddAlias("-d");
durationOption.AddValidator(result =>
{
	if(result.GetValueForOption(durationOption) < 1)
	{
		result.ErrorMessage = "CPU stress duration must be grater than 0 seconds.";
	}
});

var cpuLoadOption = new Option<int>(name: "--percentage", description: "CPU utilization percentage during the CPU stress.", getDefaultValue: () => 100);
cpuLoadOption.AddAlias("-p");
cpuLoadOption.AddValidator(result =>
{
	if(result.GetValueForOption(cpuLoadOption) < 1)
	{
		result.ErrorMessage = "CPU utilization must be greater than 0%.";
	}

  if (result.GetValueForOption(cpuLoadOption) > 100)
  {
		result.ErrorMessage = "CPU utilization must be less or equal to 100%.";
	}
});


var coresOption = new Option<int>(name: "--cores", description: "Number of CPU cores to target for the duration of the CPU stress.", getDefaultValue: () => Environment.ProcessorCount);
coresOption.AddAlias("-c");

coresOption.AddValidator(result =>
{
	if(result.GetValueForOption(coresOption) < 1)
	{
		result.ErrorMessage = "Number of targeted CPU cores must be greater than 0.";
	}

	if(result.GetValueForOption(coresOption) > Environment.ProcessorCount)
	{
		result.ErrorMessage = $"Number of targeted CPU cores must not be greater than the number of cores available ({Environment.ProcessorCount}).";
	}
});

var priorityOption = new Option<ProcessPriorityClass>("--priority", description: "Priority of the CPU stress utility.", getDefaultValue: () => ProcessPriorityClass.AboveNormal);

rootCommand.AddOption(durationOption);
rootCommand.AddOption(cpuLoadOption);
rootCommand.AddOption(coresOption);
rootCommand.AddOption(priorityOption);

rootCommand.SetHandler((cpuStressOptions) =>
{
	Console.WriteLine($"CPU stress initiated.\n{cpuStressOptions.ToString()}");
	Process currentProcess = Process.GetCurrentProcess();
	currentProcess.PriorityClass = cpuStressOptions.Priority;
	nint affinityMask = 0;
	for(int i = 0; i < cpuStressOptions.Cores; i++)
	{
		affinityMask |= 1 << i;
	}
  currentProcess.ProcessorAffinity = affinityMask;
	CpuStress.Stress(cpuStressOptions).GetAwaiter().GetResult();

	Console.WriteLine("CPU stress stopped.");
	return Task.FromResult(0);
}, new CpuStressOptionsBinder(durationOption, cpuLoadOption, coresOption, priorityOption));

await rootCommand.InvokeAsync(args);
