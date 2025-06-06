namespace SteadybitCpuStress
{
  internal class CpuStressState
  {
		public required CpuStressOptions Options { get; set; }

		public volatile bool ExecutionDone = false; 
  }
}
