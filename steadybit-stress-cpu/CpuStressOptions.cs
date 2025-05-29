using System.CommandLine;
using System.CommandLine.Binding;
using System.Diagnostics;

namespace SteadybitCpuStress
{
  internal class CpuStressOptions
  {
		public int Duration { get; set; }
		public int CpuLoad { get; set; }
		public int Cores { get; set; }

		public ProcessPriorityClass Priority { get; set; }

		public override string? ToString()
		{
			return $"Duration: {Duration}s.\nCPU load: {CpuLoad}%.\nCores: {Cores}.\nPriority: {Priority.ToString()}.";
		}
  }

  internal class CpuStressOptionsBinder : BinderBase<CpuStressOptions>
  {
		private readonly Option<int> _durationOption;
		private readonly Option<int> _cpuLoadOption;
		private readonly Option<int> _coresOption;
		private readonly Option<ProcessPriorityClass> _priorityOption;

		public CpuStressOptionsBinder(Option<int> durationOption, Option<int> cpuLoadOption, Option<int> coresOption, Option<ProcessPriorityClass> priorityOption)
		{
			_durationOption = durationOption;
			_cpuLoadOption = cpuLoadOption;
			_coresOption = coresOption;
			_priorityOption = priorityOption;
		}
		protected override CpuStressOptions GetBoundValue(BindingContext bindingContext) => new CpuStressOptions
		{
			Duration = bindingContext.ParseResult.GetValueForOption(_durationOption),
			CpuLoad = bindingContext.ParseResult.GetValueForOption(_cpuLoadOption),
			Cores = bindingContext.ParseResult.GetValueForOption(_coresOption),
			Priority = bindingContext.ParseResult.GetValueForOption(_priorityOption)
		};
  }
}
