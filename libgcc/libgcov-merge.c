/* Routines required for instrumenting a program.  */
/* Compile this one with gcc.  */
/* Copyright (C) 1989-2019 Free Software Foundation, Inc.

This file is part of GCC.

GCC is free software; you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free
Software Foundation; either version 3, or (at your option) any later
version.

GCC is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or
FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public License
for more details.

Under Section 7 of GPL version 3, you are granted additional
permissions described in the GCC Runtime Library Exception, version
3.1, as published by the Free Software Foundation.

You should have received a copy of the GNU General Public License and
a copy of the GCC Runtime Library Exception along with this program;
see the files COPYING3 and COPYING.RUNTIME respectively.  If not, see
<http://www.gnu.org/licenses/>.  */

#include "libgcov.h"

#if defined(inhibit_libc)
/* If libc and its header files are not available, provide dummy functions.  */

#ifdef L_gcov_merge_add
void __gcov_merge_add (gcov_type *counters  __attribute__ ((unused)),
                       unsigned n_counters __attribute__ ((unused))) {}
#endif

#ifdef L_gcov_merge_single
void __gcov_merge_single (gcov_type *counters  __attribute__ ((unused)),
			  unsigned n_counters __attribute__ ((unused))) {}
#endif

#else

#ifdef L_gcov_merge_add
/* The profile merging function that just adds the counters.  It is given
   an array COUNTERS of N_COUNTERS old counters and it reads the same number
   of counters from the gcov file.  */
void
__gcov_merge_add (gcov_type *counters, unsigned n_counters)
{
  for (; n_counters; counters++, n_counters--)
    *counters += gcov_get_counter ();
}
#endif /* L_gcov_merge_add */

#ifdef L_gcov_merge_ior
/* The profile merging function that just adds the counters.  It is given
   an array COUNTERS of N_COUNTERS old counters and it reads the same number
   of counters from the gcov file.  */
void
__gcov_merge_ior (gcov_type *counters, unsigned n_counters)
{
  for (; n_counters; counters++, n_counters--)
    *counters |= gcov_get_counter_target ();
}
#endif

#ifdef L_gcov_merge_time_profile
/* Time profiles are merged so that minimum from all valid (greater than zero)
   is stored. There could be a fork that creates new counters. To have
   the profile stable, we chosen to pick the smallest function visit time.  */
void
__gcov_merge_time_profile (gcov_type *counters, unsigned n_counters)
{
  unsigned int i;
  gcov_type value;

  for (i = 0; i < n_counters; i++)
    {
      value = gcov_get_counter_target ();

      if (value && (!counters[i] || value < counters[i]))
        counters[i] = value;
    }
}
#endif /* L_gcov_merge_time_profile */

#ifdef L_gcov_merge_single

static void
merge_single_value_set (gcov_type *counters)
{
  unsigned j;
  gcov_type value, counter;

  /* First value is number of total executions of the profiler.  */
  gcov_type all = gcov_get_counter_ignore_scaling (-1);
  counters[0] += all;
  ++counters;

  for (unsigned i = 0; i < GCOV_DISK_SINGLE_VALUES; i++)
    {
      value = gcov_get_counter_target ();
      counter = gcov_get_counter_ignore_scaling (-1);

      if (counter == -1)
	{
	  counters[1] = -1;
	  /* We can't return as we need to read all counters.  */
	  continue;
	}
      else if (counter == 0 || counters[1] == -1)
	{
	  /* We can't return as we need to read all counters.  */
	  continue;
	}

      for (j = 0; j < GCOV_DISK_SINGLE_VALUES; j++)
	{
	  if (counters[2 * j] == value)
	    {
	      counters[2 * j + 1] += counter;
	      break;
	    }
	  else if (counters[2 * j + 1] == 0)
	    {
	      counters[2 * j] = value;
	      counters[2 * j + 1] = counter;
	      break;
	    }
	}

      /* We haven't found a free slot for the value, mark overflow.  */
      if (j == GCOV_DISK_SINGLE_VALUES)
	counters[1] = -1;
    }
}

/* The profile merging function for choosing the most common value.
   It is given an array COUNTERS of N_COUNTERS old counters and it
   reads the same number of counters from the gcov file.  The counters
   are split into pairs where the members of the tuple have
   meanings:

   -- the stored candidate on the most common value of the measured entity
   -- counter
   */
void
__gcov_merge_single (gcov_type *counters, unsigned n_counters)
{
  gcc_assert (!(n_counters % GCOV_SINGLE_VALUE_COUNTERS));

  for (unsigned i = 0; i < (n_counters / GCOV_SINGLE_VALUE_COUNTERS); i++)
    merge_single_value_set (counters + (i * GCOV_SINGLE_VALUE_COUNTERS));
}
#endif /* L_gcov_merge_single */

#endif /* inhibit_libc */
