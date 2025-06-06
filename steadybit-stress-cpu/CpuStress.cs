using System.Diagnostics;

namespace SteadybitCpuStress
{
  internal class CpuStress
	{
		public static async Task Stress(CpuStressOptions options)
		{
			List<Thread> threads = new List<Thread>();
			CpuStressState state = new CpuStressState { Options = options };
			for (int i = 0; i < options.Cores; i++)
			{
				Thread t = new Thread(new ParameterizedThreadStart(CoreStress));
				t.Start(state);
				threads.Add(t);
			}

			Thread.Sleep(options.Duration * 1000);
			state.ExecutionDone = true;
			foreach (Thread t in threads)
			{
				t.Join();
			}
		}

		static void CoreStress(object obj)
		{
			CpuStressState state = (CpuStressState)obj;

			Stopwatch sw = Stopwatch.StartNew();
			var interval = 100;

			sw.Start();
			while (true)
			{
				if (state.ExecutionDone)
				{
					break;
				}
				sw.Restart();

				while (sw.ElapsedMilliseconds < state.Options.CpuLoad * interval / 100.0)
				{
					Thread.SpinWait(100);
				}

				int sleepTime = (int)(interval - state.Options.CpuLoad * interval / 100.0);
				if (sleepTime > 0)
				{
					Thread.Sleep(sleepTime);
				}
			}
		}
	}
}
